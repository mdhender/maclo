// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm_test

import (
	"errors"
	"testing"

	"github.com/mdhender/maclo/pkg/lowl/op"
	"github.com/mdhender/maclo/pkg/lowl/vm"
)

// TestWorkspaceStartsAfterTheProgram pins where the free space is.
//
// It used to start at address zero, which is inside the program: the first
// instruction of ML/I loads FFPT and carves its permanent variables out of the
// space it points at, so the first allocation wrote over the reserved words
// and then over the logic. Nothing failed at that point; it went wrong much
// later and a long way away.
func TestWorkspaceStartsAfterTheProgram(t *testing.T) {
	m := vm.New()
	// stand in for an assembled program: a variable, then some logic.
	ffpt := m.Registers.FFPT
	lfpt := m.Registers.LFPT
	m.Registers.Last = 400

	m.SetWorkspace(0, 128)

	base := m.Core[ffpt].Value
	if base <= m.Registers.Last {
		t.Errorf("ffpt: want an address past the program (%d): got %d", m.Registers.Last, base)
	}
	if got, want := m.Core[lfpt].Value, base+128; got != want {
		t.Errorf("lfpt: want %d: got %d", want, got)
	}

	// a host that knows its own layout must be able to say so
	m.SetWorkspace(2_000, 64)
	if got := m.Core[ffpt].Value; got != 2_000 {
		t.Errorf("ffpt: want the base it was given (2000): got %d", got)
	}
	if got := m.Core[lfpt].Value; got != 2_064 {
		t.Errorf("lfpt: want 2064: got %d", got)
	}
}

// TestSystemVariableLayout checks the block against the way ML/I reads it.
//
// Supplement 3 stores the S-variables in reverse, S1 last, with a count above
// them that SVARPT points at. The ML/I source never writes any of this, so a
// layout that is wrong here is wrong everywhere, and the code that reads it
// looks like "LBV SVARPT; SBL OF(3*LNM); LAM 0" — load B with the value of
// SVARPT, step back three numbers, take what is there. That is what this
// checks, one S-variable at a time.
func TestSystemVariableLayout(t *testing.T) {
	m := vm.New()
	m.Registers.Last = 400
	m.Symbols["SVARPT"] = 20

	// distinguishable values, so that an off-by-one shows up as the wrong
	// variable rather than as the wrong number.
	values := make([]int, 24)
	for i := range values {
		values[i] = 100 + i + 1 // S1 is 101, S24 is 124
	}

	next, err := m.SetSystemVariables(0, values)
	if err != nil {
		t.Fatalf("svars: %v", err)
	}

	svarpt := m.Core[20].Value
	if got, want := m.Core[svarpt].Value, len(values); got != want {
		t.Errorf("svars: count: want %d: got %d", want, got)
	}
	for n := 1; n <= len(values); n++ {
		address := svarpt - n*m.Registers.LNM
		if got, want := m.Core[address].Value, 100+n; got != want {
			t.Errorf("svars: S%d: want %d: got %d", n, want, got)
		}
	}

	// the block sits after the program and before the workspace the caller
	// lays out behind it.
	base := svarpt - len(values)*m.Registers.LNM
	if base <= m.Registers.Last {
		t.Errorf("svars: want a base past the program (%d): got %d", m.Registers.Last, base)
	}
	if want := svarpt + m.Registers.LNM; next != want {
		t.Errorf("svars: next: want %d: got %d", want, next)
	}
}

// TestPseudoAlphanumericCharacter covers S6, which GOPC is the only reader of.
//
// ML/I splits text into atoms with GOPC and nothing else, and S6 names one
// character that is to be counted as alphanumeric even though it is neither a
// letter nor a digit, so that it can appear inside an atom. The MI-logic never
// looks at S6 — the whole feature is the machine's — and it can be assigned to
// between any two characters, so the answer has to be read afresh each time.
func TestPseudoAlphanumericCharacter(t *testing.T) {
	// GOPC at address 90, branching to 99 when the character is punctuation.
	newMachine := func(t *testing.T) *vm.VM {
		t.Helper()
		m := vm.New()
		m.Registers.Last = 400
		m.Symbols["SVARPT"] = 20
		if _, err := m.SetSystemVariables(0, systemVariablesWithS6(-1)); err != nil {
			t.Fatalf("svars: %v", err)
		}
		m.SetWord(90, vm.Word{Op: op.GOPC, Value: 99})
		return m
	}
	branches := func(t *testing.T, m *vm.VM, ch int) bool {
		t.Helper()
		m.PC, m.C = 90, ch
		if err := m.Step(nil, nil); err != nil {
			t.Fatalf("step: %v", err)
		}
		return m.PC == 99
	}

	// with no S6, the kernel's own definition: a letter or a digit is not
	// punctuation and everything else is.
	m := newMachine(t)
	for _, ch := range []int{'a', 'Z', '7'} {
		if branches(t, m, ch) {
			t.Errorf("S6 unset: %q: want alphanumeric: got punctuation", ch)
		}
	}
	for _, ch := range []int{'_', '$', ' ', '\n'} {
		if !branches(t, m, ch) {
			t.Errorf("S6 unset: %q: want punctuation: got alphanumeric", ch)
		}
	}

	// setting S6 takes effect on the machine that is already running, because
	// a macro is what sets it.
	svarpt := m.Core[20].Value
	m.Core[svarpt-6*m.Registers.LNM].Value = '_'
	if branches(t, m, '_') {
		t.Errorf("S6 = '_': want alphanumeric: got punctuation")
	}
	if !branches(t, m, '$') {
		t.Errorf("S6 = '_': %q: want punctuation: got alphanumeric", '$')
	}

	// a program with no S-variables at all, such as LOWLTEST, gets the plain
	// definition rather than reading address zero.
	bare := vm.New()
	bare.SetWord(90, vm.Word{Op: op.GOPC, Value: 99})
	if !branches(t, bare, '_') {
		t.Errorf("no S-variables: want punctuation: got alphanumeric")
	}
}

// systemVariablesWithS6 is the smallest block that has an S6 in it.
func systemVariablesWithS6(value int) []int {
	values := make([]int, vm.MinSystemVariables)
	values[6-1] = value
	return values
}

// TestSystemVariablesRefused covers the two ways the block cannot be built.
func TestSystemVariablesRefused(t *testing.T) {
	m := vm.New()
	if _, err := m.SetSystemVariables(0, make([]int, 24)); !errors.Is(err, vm.ErrNoSymbol) {
		t.Errorf("no SVARPT: want ErrNoSymbol: got %v", err)
	}

	m.Symbols["SVARPT"] = 20
	if _, err := m.SetSystemVariables(0, make([]int, vm.MinSystemVariables-1)); !errors.Is(err, vm.ErrSystemVariables) {
		t.Errorf("too few: want ErrSystemVariables: got %v", err)
	}
}

// TestRunKeepsTheHostsWorkspace checks that Run does not overwrite a layout the
// host has already made. ML/I reads the S-variables out of the workspace, so a
// Run that reset FFPT would move the floor out from under them.
func TestRunKeepsTheHostsWorkspace(t *testing.T) {
	m := vm.New()
	m.Registers.Last = 100
	m.Registers.Start = 90
	m.SetWord(90, vm.Word{Op: op.MDQUIT}) // the graceful exit, so Run returns nil
	m.SetWorkspace(3_000, 256)

	if err := m.Run(nil, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := m.Core[m.Registers.FFPT].Value; got != 3_000 {
		t.Errorf("ffpt: want the host's 3000: got %d", got)
	}
}

// TestRunCycleLimit checks that the ceiling is a knob rather than a constant.
// Real ML/I input runs for far longer than the default allows.
func TestRunCycleLimit(t *testing.T) {
	m := vm.New()
	m.Registers.Start = 90
	m.Registers.Last = 100
	// a two word loop that never stops
	m.SetWord(90, vm.Word{Op: op.NOOP})
	m.SetWord(91, vm.Word{Op: op.GO, Value: 90})

	m.Registers.Cycles = 32
	if err := m.Run(nil, nil); err != vm.ErrCycles {
		t.Errorf("run: want ErrCycles: got %v", err)
	}
}
