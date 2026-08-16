// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm

import (
	"errors"
	"fmt"
	"io"
)

// DefaultWorkspace is how many words Run gives a program that has not had a
// workspace set for it. It is a fallback for running a LOWL program on its
// own, such as LOWLTEST; a host that knows how much the user asked for should
// call SetWorkspace instead.
const DefaultWorkspace = 5_000

// DefaultCycles is the ceiling Run puts on how long a program may execute.
// Real ML/I input runs for far longer than a test program does, so a host that
// expects a long run should raise Registers.Cycles.
const DefaultCycles = 10_000

// SetWorkspace lays out the free space a LOWL program allocates from and
// points FFPT and LFPT at its two ends.
//
// The forwards stack grows up from FFPT and the backwards stack grows down
// from LFPT, and both live in core: FSTK, BSTK and CFSTK reach them through
// those two variables, so the workspace has to be core addresses rather than a
// separate array. It starts after the program because the program is in core
// too, and a workspace that began at address zero would have the first
// allocation write over the reserved words and then over the logic itself.
//
// base is the first address the program may use. Passing zero starts the
// workspace immediately after the assembled program, which is what a program
// with no host-supplied storage wants.
func (m *VM) SetWorkspace(base, words int) {
	if base == 0 {
		base = m.Registers.Last + 1
	}
	if words <= 0 {
		words = DefaultWorkspace
	}
	if m.Registers.FFPT != 0 {
		m.directStore(m.Registers.FFPT, base)
	}
	if m.Registers.LFPT != 0 {
		m.directStore(m.Registers.LFPT, base+words)
	}
}

// MinSystemVariables is the smallest S-variable block Supplement 3 allows.
// S1 to S9 belong to ML/I itself; everything above them is the host's.
const MinSystemVariables = 9

// SetSystemVariables lays the S-variable block out in core and points SVARPT
// at it.
//
// SVARPT is the one variable ML/I reads and never writes, so nothing in the
// program will build this block: it has to exist before the first instruction
// runs, or the first access computes a negative address. Supplement 3 fixes
// the shape. The variables are stored in reverse, S1 last, immediately
// followed by a word holding how many there are, and SVARPT points at that
// count rather than at the first variable:
//
//	Core[svarpt]         the count
//	Core[svarpt - n*LNM] Sn
//
// values[0] is S1. base is the first address of the block; passing zero puts
// it immediately after the assembled program. The return value is the first
// address after the block, which is where the caller should start the
// workspace.
func (m *VM) SetSystemVariables(base int, values []int) (int, error) {
	svarpt, ok := m.Symbols["SVARPT"]
	if !ok {
		return 0, fmt.Errorf("SVARPT: %w", ErrNoSymbol)
	}
	if len(values) < MinSystemVariables {
		return 0, fmt.Errorf("%d variables: %w (at least %d)", len(values), ErrSystemVariables, MinSystemVariables)
	}
	if base == 0 {
		base = m.Registers.Last + 1
	}

	// the count sits above the variables, at the top of the block, because
	// that is the end SVARPT points at.
	count := base + len(values)*m.Registers.LNM
	m.directStore(svarpt, count)
	m.directStore(count, len(values))
	for n, value := range values {
		m.directStore(count-(n+1)*m.Registers.LNM, value)
	}

	// GOPC reads S6 out of the block it has just been given, and remembering
	// where SVARPT is saves it a lookup for every character of the source text.
	m.Registers.SVARPT = svarpt
	return count + m.Registers.LNM, nil
}

func (m *VM) Run(fp, msg io.Writer) error {
	m.PC = m.Registers.Start
	m.Streams.Stdout = fp
	m.Streams.Messages = msg

	// A host that has already laid out its own storage will have set FFPT, and
	// its layout must win: it knows where it put things that the program is
	// about to read, such as the S-variables.
	if m.Registers.FFPT != 0 && m.directLoad(m.Registers.FFPT) == 0 {
		m.SetWorkspace(0, DefaultWorkspace)
	}

	// this is commentary about the machine, not output of the program, so it
	// goes to the trace stream. The messages stream is the program's.
	printf(m.Streams.Trace, "vm: starting %d\n", m.Registers.Start)
	m.Registers.Halted = false
	cycles := m.Registers.Cycles
	if cycles <= 0 {
		cycles = DefaultCycles
	}
	for ; !m.Registers.Halted && cycles > 0; cycles-- {
		if err := m.Step(fp, msg); err != nil {
			if !errors.Is(err, ErrQuit) {
				return err
			}
			// graceful exit; cleanup and return happy
			return nil
		}
	}
	return ErrCycles
}
