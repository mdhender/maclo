// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm_test

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/maloquacious/ml_i/pkg/lowl/op"
	"github.com/maloquacious/ml_i/pkg/lowl/vm"
	"io"
	"strings"
	"testing"
)

type input_t struct {
	PC      int
	A, B, C int
	Cmp     vm.CMPRSLT
	RS      []int
	Text    string
	V, V2   val_t
}
type expect_t struct {
	PC      int
	A, B, C int
	Cmp     vm.CMPRSLT
	RS      []int
	Text    string
	V, V2   val_t
}

type val_t struct {
	address int
	value   int
}

// TestVM tests running, stepping, and all the op codes in the machine.
func TestVM(t *testing.T) {
	var m *vm.VM
	var opc op.Code
	var input input_t
	var expect expect_t
	var out *bytes.Buffer
	var buf []byte

	newvm := func() {
		m = &vm.VM{PC: input.PC, A: input.A, B: input.B, C: input.C}
		m.Registers.Cmp = input.Cmp
		m.RS = append(m.RS, input.RS...)
		if input.V.address != 0 {
			m.SetWord(input.V.address, vm.Word{Op: op.CON, Value: input.V.value})
		}
		if input.V2.address != 0 {
			m.SetWord(input.V2.address, vm.Word{Op: op.CON, Value: input.V2.value})
		}
	}
	// stackvm is newvm for the four statements that use the double stack.
	//
	// A bare VM will not do for those. They reach the free space through FFPT
	// and LFPT, which are reserved addresses that vm.New lays out, so on a
	// machine built by hand both pointers are address zero — which is the HALT
	// that ends a program, not storage. The workspace is a block of core
	// running from base, with the forwards stack growing up from FFPT at one
	// end and the backwards stack down from LFPT at the other.
	stackvm := func(base, words int) {
		m = vm.New()
		m.A, m.B, m.C = input.A, input.B, input.C
		m.Registers.Cmp = input.Cmp
		m.RS = append(m.RS, input.RS...)
		m.Registers.Last = base - 1
		m.SetWorkspace(base, words)
		m.PC = input.PC
	}
	// ffpt and lfpt read back the two ends of the workspace.
	ffpt := func() int { return m.Core[m.Registers.FFPT].Value }
	lfpt := func() int { return m.Core[m.Registers.LFPT].Value }
	testWord := func(what string, address, want int) {
		if got := m.Core[address].Value; got != want {
			t.Errorf("%s: %s: [%d]: want %d: got %d\n", opc, what, address, want, got)
		}
	}
	testJumpValue := func(want int) {
		if got := m.Registers.JumpValue; got != want {
			t.Errorf("%s: jump value: want %d: got %d\n", opc, want, got)
		}
	}
	testOverflows := func() {
		if err := m.Step(nil, nil); !errors.Is(err, vm.ErrStackOverflow) {
			t.Errorf("%s: want stack overflow: got %v\n", opc, err)
		}
	}

	step := func(stdout, stdmsg io.Writer) {
		if err := m.Step(stdout, stdmsg); err != nil {
			t.Errorf("%s: want nil: got %v\n", opc, err)
		}
	}
	testA := func() {
		if m.A != expect.A {
			t.Errorf("%s: r.A: want %d: got %d\n", opc, expect.A, m.A)
		}
	}
	testB := func() {
		if m.B != expect.B {
			t.Errorf("%s: r.B: want %d: got %d\n", opc, expect.B, m.B)
		}
	}
	testC := func() {
		if m.C != expect.C {
			t.Errorf("%s: r.C: want %d: got %d\n", opc, expect.C, m.C)
		}
	}
	testCmpResult := func() {
		if m.Registers.Cmp != expect.Cmp {
			t.Errorf("%s: cmp: want %q: got %q\n", opc, expect.Cmp, m.Registers.Cmp)
		}
	}
	testPC := func() {
		if m.PC != expect.PC {
			t.Errorf("%s: pc: want %d: got %d\n", opc, expect.PC, m.PC)
		}
	}
	testRS := func() {
		want := fmt.Sprintf("%v", expect.RS)
		got := fmt.Sprintf("%v", m.RS)
		if got != want {
			t.Errorf("%s: rs: want %s: got %s\n", opc, want, got)
		}
	}
	testV := func() {
		if expect.V.address != 0 {
			valOfV := m.Core[expect.V.address].Value
			if valOfV != expect.V.value {
				t.Errorf("%s: *v: want %d: got %d\n", opc, expect.V.value, valOfV)
			}
		}
	}
	test := func(stdout, stdmsg io.Writer) {
		step(stdout, stdmsg)
		testA()
		testB()
		testC()
		testPC()
		testRS()
		testCmpResult()
		testV()
	}

	// halting test must pass before running further tests
	opc = op.HALT
	input = input_t{}
	expect = expect_t{}
	newvm()
	if m.PC != expect.PC || m.Core[0].Op != opc {
		t.Fatalf("setw: want pc %d op HALT: got %d %q\n", expect.PC, m.PC, m.Core[m.PC].Op)
	}
	if err := m.Run(nil, nil); err != nil {
		if !errors.Is(err, vm.ErrHalted) {
			t.Fatalf("run: want ErrHalt: got %v\n", err)
		}
	}
	if m.PC != expect.PC {
		t.Fatalf("pc: wants %d: got %d\n", expect.PC, m.PC)
	}

	opc = op.AAL
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}}
	expect = expect_t{PC: 1, A: input.A + input.V.value, B: input.B, C: input.C}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.value})
	test(nil, nil)

	opc = op.AAV
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}}
	expect = expect_t{PC: 1, A: input.A + input.V.value, B: input.B, C: input.C, V: input.V}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)

	opc = op.ABV
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}}
	expect = expect_t{PC: 1, A: input.A, B: input.B + input.V.value, C: input.C, V: val_t{1, input.V.value}}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)

	opc = op.ALIGN
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want invalid op: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrInvalidOp) {
		t.Errorf("%s: want invalid op: got %v\n", opc, err)
	}

	opc = op.ANDL
	input = input_t{A: 3, B: 12, C: 49, V: val_t{1, 11}}
	expect = expect_t{PC: 1, A: input.A & input.V.value, B: input.B, C: input.C, V: input.V}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.value})
	test(nil, nil)

	opc = op.ANDV
	input = input_t{A: 3, B: 12, C: 49, V: val_t{1, 11}}
	expect = expect_t{PC: 1, A: input.A & input.V.value, B: input.B, C: input.C, V: input.V}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)

	opc = op.BMOVE
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	test(nil, nil)
	buf = []byte{}
	input = input_t{A: len(buf), V: val_t{address: 1, value: 16}, V2: val_t{address: 2, value: 21}}
	expect = expect_t{PC: 1, A: len(buf)}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address, ValueTwo: input.V2.address})
	m.SetWord(input.V.address, vm.Word{Value: input.V.value})
	m.SetWord(input.V2.address, vm.Word{Value: input.V2.value})
	for n, ch := range buf {
		m.SetWord(input.V.value+n, vm.Word{Value: int(ch)})
	}
	test(nil, nil)
	for n, want := range buf {
		got := byte(m.Core[input.V2.value+n].Value)
		if got != want {
			t.Errorf("%s: [%d]: want %d: got %d\n", opc, n, want, got)
		}
	}
	buf = []byte{138}
	input = input_t{A: len(buf), V: val_t{address: 1, value: 16}, V2: val_t{address: 2, value: 21}}
	expect = expect_t{PC: 1, A: len(buf)}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address, ValueTwo: input.V2.address})
	m.SetWord(input.V.address, vm.Word{Value: input.V.value})
	m.SetWord(input.V2.address, vm.Word{Value: input.V2.value})
	for n, ch := range buf {
		m.SetWord(input.V.value+n, vm.Word{Value: int(ch)})
	}
	test(nil, nil)
	for n, want := range buf {
		got := byte(m.Core[input.V2.value+n].Value)
		if got != want {
			t.Errorf("%s: [%d]: want %d: got %d\n", opc, n, want, got)
		}
	}
	buf = []byte{98, 97, 4, 3, 2, 1, 0}
	input = input_t{A: len(buf), V: val_t{address: 1, value: 16}, V2: val_t{address: 2, value: 21}}
	expect = expect_t{PC: 1, A: len(buf)}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address, ValueTwo: input.V2.address})
	m.SetWord(input.V.address, vm.Word{Value: input.V.value})
	m.SetWord(input.V2.address, vm.Word{Value: input.V2.value})
	for n, ch := range buf {
		m.SetWord(input.V.value+n, vm.Word{Value: int(ch)})
	}
	test(nil, nil)
	for n, want := range buf {
		got := byte(m.Core[input.V2.value+n].Value)
		if got != want {
			t.Errorf("%s: [%d]: want %d: got %d\n", opc, n, want, got)
		}
	}

	// BSTK stacks A on the backwards stack, which grows down from LFPT. The
	// manual maps it as: step LFPT back one number, store A there, then load A
	// with FFPT and compare the two — so A is not preserved, it comes out
	// holding the other end of the stack.
	opc = op.BSTK
	input = input_t{PC: 40, A: 1234, B: 7, C: 'x'}
	expect = expect_t{PC: 41, A: 200, B: input.B, C: input.C, Cmp: vm.IS_LT}
	stackvm(200, 64)
	m.SetWord(input.PC, vm.Word{Op: opc})
	test(nil, nil)
	testWord("stacked", 263, input.A)
	if lfpt() != 263 {
		t.Errorf("%s: lfpt: want 263: got %d\n", opc, lfpt())
	}
	// stacking twice puts the second item below the first
	m.A = 5678
	m.PC = input.PC
	step(nil, nil)
	testWord("stacked first", 263, 1234)
	testWord("stacked second", 262, 5678)

	// the two stacks meeting is an overflow, and it is detected after the
	// store rather than before it
	input = input_t{PC: 40, A: 1234}
	stackvm(200, 1)
	m.SetWord(input.PC, vm.Word{Op: opc})
	testOverflows()

	opc = op.BUMP
	input = input_t{A: 3, B: 12, C: 49, V: val_t{1, 11}, V2: val_t{value: 2}}
	expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.C, V: val_t{input.V.address, input.V.value + input.V2.value}}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address, ValueTwo: input.V2.value})
	test(nil, nil)

	opc = op.CAI
	for _, tc := range []struct {
		a, i int
		cmp  vm.CMPRSLT
	}{
		{23, 29, vm.IS_LT},
		{29, 29, vm.IS_EQ},
		{31, 29, vm.IS_GR},
	} {
		input = input_t{A: tc.a, B: 4, C: 5, V: val_t{1, 8}, V2: val_t{8, tc.i}}
		expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.C, V: input.V, V2: input.V2, Cmp: tc.cmp}
		newvm()
		m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
		test(nil, nil)
	}

	opc = op.CAL
	for _, tc := range []struct {
		a, i int
		cmp  vm.CMPRSLT
	}{
		{23, 29, vm.IS_LT},
		{29, 29, vm.IS_EQ},
		{31, 29, vm.IS_GR},
	} {
		input = input_t{A: tc.a, B: 4, C: 5, V: val_t{1, tc.i}}
		expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.C, V: input.V, Cmp: tc.cmp}
		newvm()
		m.SetWord(0, vm.Word{Op: opc, Value: input.V.value})
		test(nil, nil)
	}

	opc = op.CAV
	for _, tc := range []struct {
		a, i int
		cmp  vm.CMPRSLT
	}{
		{23, 29, vm.IS_LT},
		{29, 29, vm.IS_EQ},
		{31, 29, vm.IS_GR},
	} {
		input = input_t{A: tc.a, B: 4, C: 5, V: val_t{1, tc.i}}
		expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.C, V: input.V, Cmp: tc.cmp}
		newvm()
		m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
		test(nil, nil)
	}

	opc = op.CCI
	for _, tc := range []struct {
		c, i int
		cmp  vm.CMPRSLT
	}{
		{23, 29, vm.IS_LT},
		{29, 29, vm.IS_EQ},
		{31, 29, vm.IS_GR},
	} {
		input = input_t{A: 13, B: 4, C: tc.c, V: val_t{1, 8}, V2: val_t{8, tc.i}}
		expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.C, V: input.V, V2: input.V2, Cmp: tc.cmp}
		newvm()
		m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
		test(nil, nil)
	}

	opc = op.CCL
	for _, tc := range []struct {
		c, i int
		cmp  vm.CMPRSLT
	}{
		{23, 29, vm.IS_LT},
		{29, 29, vm.IS_EQ},
		{31, 29, vm.IS_GR},
	} {
		input = input_t{A: 15, B: 4, C: tc.c, V: val_t{1, tc.i}}
		expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.C, V: input.V, Cmp: tc.cmp}
		newvm()
		m.SetWord(0, vm.Word{Op: opc, Value: input.V.value})
		test(nil, nil)
	}

	opc = op.CCN
	for _, tc := range []struct {
		c, i int
		cmp  vm.CMPRSLT
	}{
		{23, 29, vm.IS_LT},
		{29, 29, vm.IS_EQ},
		{31, 29, vm.IS_GR},
	} {
		input = input_t{A: 15, B: 4, C: tc.c, V: val_t{1, tc.i}}
		expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.C, V: input.V, Cmp: tc.cmp}
		newvm()
		m.SetWord(0, vm.Word{Op: opc, Value: input.V.value})
		test(nil, nil)
	}

	// CFSTK is FSTK with two differences the manual states in one sentence:
	// what is stacked is C rather than A, and FFPT is stepped by the length of
	// a character rather than the length of a number.
	opc = op.CFSTK
	input = input_t{PC: 40, A: 9, B: 7, C: 'K'}
	expect = expect_t{PC: 41, A: 201, B: input.B, C: input.C, Cmp: vm.IS_LT}
	stackvm(200, 64)
	m.SetWord(input.PC, vm.Word{Op: opc})
	test(nil, nil)
	testWord("stacked", 200, 'K')
	if ffpt() != 201 {
		t.Errorf("%s: ffpt: want 201: got %d\n", opc, ffpt())
	}

	// LCH and LNM are both one word on this machine, so the two are only told
	// apart by a machine where they differ. FSTK steps by LNM and CFSTK by
	// LCH, and using the wrong one is invisible until then.
	input = input_t{PC: 40, C: 'K'}
	stackvm(200, 64)
	m.Registers.LCH, m.Registers.LNM = 2, 3
	m.SetWord(input.PC, vm.Word{Op: opc})
	step(nil, nil)
	if ffpt() != 202 {
		t.Errorf("%s: ffpt: want 202 (stepped by LCH): got %d\n", opc, ffpt())
	}

	input = input_t{PC: 40, C: 'K'}
	stackvm(200, 1)
	m.SetWord(input.PC, vm.Word{Op: opc})
	testOverflows()

	opc = op.CLEAR
	input = input_t{A: 3, B: 12, C: 49, V: val_t{1, 11}}
	expect = expect_t{PC: 1, A: input.A & input.V.value, B: input.B, C: input.C, V: val_t{input.V.address, input.V.value}}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.value})
	test(nil, nil)

	opc = op.CON
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want invalid op: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrInvalidOp) {
		t.Errorf("%s: want invalid op: got %v\n", opc, err)
	}

	// CSS is Clear Subroutine Stack. It marks a label in the main logic that
	// something branches to from inside a subroutine, and that branch can come
	// from any depth, so it empties the stack rather than popping one link.
	// An empty stack is not an error: the same label is also fallen into.
	opc = op.CSS
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	test(nil, nil)

	opc = op.CSS
	input = input_t{RS: []int{97, 98, 99}}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	test(nil, nil)

	opc = op.DCL
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want invalid op: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrInvalidOp) {
		t.Errorf("%s: want invalid op: got %v\n", opc, err)
	}

	opc = op.EQU
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want invalid op: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrInvalidOp) {
		t.Errorf("%s: want invalid op: got %v\n", opc, err)
	}

	// EXIT N returns from a subroutine and says which of its exits was taken.
	// Exit 1 is the plain return, exit 2 returns and skips the statement after
	// the call, and so on; the assembler turns those skipped statements into
	// the GOTBL words that compare themselves against the number left here.
	opc = op.EXIT
	input = input_t{PC: 40, A: 3, B: 4, C: 5, RS: []int{77}}
	expect = expect_t{PC: 77, A: input.A, B: input.B, C: input.C, RS: []int{}}
	newvm()
	m.SetWord(input.PC, vm.Word{Op: opc, Value: 2})
	test(nil, nil)
	testJumpValue(2)

	// nesting: the inner return address is the one that comes back first
	input = input_t{PC: 40, RS: []int{77, 88}}
	expect = expect_t{PC: 88, RS: []int{77}}
	newvm()
	m.SetWord(input.PC, vm.Word{Op: opc, Value: 1})
	test(nil, nil)
	testJumpValue(1)

	// an EXIT with nothing to return to is a fault in the program rather than
	// a place to stop, and it must be reported rather than reached for
	input = input_t{PC: 40}
	newvm()
	m.SetWord(input.PC, vm.Word{Op: opc, Value: 1})
	if err := m.Step(nil, nil); !errors.Is(err, vm.ErrStackUnderflow) {
		t.Errorf("%s: want stack underflow: got %v\n", opc, err)
	}

	opc = op.FMOVE
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	test(nil, nil)
	buf = []byte{}
	input = input_t{A: len(buf), V: val_t{address: 1, value: 16}, V2: val_t{address: 2, value: 21}}
	expect = expect_t{PC: 1, A: len(buf)}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address, ValueTwo: input.V2.address})
	m.SetWord(input.V.address, vm.Word{Value: input.V.value})
	m.SetWord(input.V2.address, vm.Word{Value: input.V2.value})
	for n, ch := range buf {
		m.SetWord(input.V.value+n, vm.Word{Value: int(ch)})
	}
	test(nil, nil)
	for n, want := range buf {
		got := byte(m.Core[input.V2.value+n].Value)
		if got != want {
			t.Errorf("%s: [%d]: want %d: got %d\n", opc, n, want, got)
		}
	}
	buf = []byte{48}
	input = input_t{A: len(buf), V: val_t{address: 1, value: 16}, V2: val_t{address: 2, value: 21}}
	expect = expect_t{PC: 1, A: len(buf)}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address, ValueTwo: input.V2.address})
	m.SetWord(input.V.address, vm.Word{Value: input.V.value})
	m.SetWord(input.V2.address, vm.Word{Value: input.V2.value})
	for n, ch := range buf {
		m.SetWord(input.V.value+n, vm.Word{Value: int(ch)})
	}
	test(nil, nil)
	for n, want := range buf {
		got := byte(m.Core[input.V2.value+n].Value)
		if got != want {
			t.Errorf("%s: [%d]: want %d: got %d\n", opc, n, want, got)
		}
	}
	buf = []byte{1, 2, 3}
	input = input_t{A: len(buf), V: val_t{address: 1, value: 16}, V2: val_t{address: 2, value: 21}}
	expect = expect_t{PC: 1, A: len(buf)}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address, ValueTwo: input.V2.address})
	m.SetWord(input.V.address, vm.Word{Value: input.V.value})
	m.SetWord(input.V2.address, vm.Word{Value: input.V2.value})
	for n, ch := range buf {
		m.SetWord(input.V.value+n, vm.Word{Value: int(ch)})
	}
	test(nil, nil)
	for n, want := range buf {
		got := byte(m.Core[input.V2.value+n].Value)
		if got != want {
			t.Errorf("%s: [%d]: want %d: got %d\n", opc, n, want, got)
		}
	}

	// FSTK stacks A on the forwards stack, which grows up from FFPT. A is
	// clobbered on the way: the manual's mapping loads it with the stepped
	// FFPT so that the overflow test has something to compare.
	opc = op.FSTK
	input = input_t{PC: 40, A: 4321, B: 7, C: 'x'}
	expect = expect_t{PC: 41, A: 201, B: input.B, C: input.C, Cmp: vm.IS_LT}
	stackvm(200, 64)
	m.SetWord(input.PC, vm.Word{Op: opc})
	test(nil, nil)
	testWord("stacked", 200, input.A)
	if ffpt() != 201 {
		t.Errorf("%s: ffpt: want 201: got %d\n", opc, ffpt())
	}
	// stacking twice puts the second item above the first
	m.A = 8765
	m.PC = input.PC
	step(nil, nil)
	testWord("stacked first", 200, 4321)
	testWord("stacked second", 201, 8765)

	input = input_t{PC: 40, A: 4321}
	stackvm(200, 1)
	m.SetWord(input.PC, vm.Word{Op: opc})
	testOverflows()

	// the two stacks share one block, so filling either one is what runs out.
	// Here the forwards stack is walked up until it reaches an untouched
	// backwards stack.
	input = input_t{PC: 40, A: 1}
	stackvm(200, 4)
	m.SetWord(input.PC, vm.Word{Op: opc})
	for i := 0; i < 3; i++ {
		m.PC = input.PC
		step(nil, nil)
	}
	m.PC = input.PC
	testOverflows()

	opc = op.GO
	input = input_t{Cmp: vm.IS_LT}
	expect = expect_t{PC: 8, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{Cmp: vm.IS_EQ}
	expect = expect_t{PC: 8, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{Cmp: vm.IS_GR}
	expect = expect_t{PC: 8, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)

	// GOADD is a multi-way branch: it is always followed by a run of
	// unconditional GO statements, and the value of its variable picks one of
	// them, counting from zero. It does not branch itself. What it does is
	// leave the number where the GOs, which the assembler turned into GOTBL
	// words, can compare themselves against it.
	opc = op.GOADD
	for _, choice := range []int{0, 1, 2} {
		input = input_t{PC: 40, A: 3, B: 4, C: 5, V: val_t{8, choice}}
		expect = expect_t{PC: 41, A: input.A, B: input.B, C: input.C, V: input.V}
		newvm()
		m.SetWord(input.PC, vm.Word{Op: opc, Value: input.V.address})
		test(nil, nil)
		testJumpValue(choice)
	}

	// the whole statement, as the assembler lays it out: GOADD and its table.
	// A GOADD table is numbered from zero, which is why "T" and "C" are
	// separate flags in the assembler.
	for _, tc := range []struct{ choice, want int }{
		{0, 500},
		{1, 501},
		{2, 502},
	} {
		input = input_t{PC: 40, V: val_t{8, tc.choice}}
		newvm()
		m.SetWord(input.PC, vm.Word{Op: opc, Value: input.V.address})
		m.SetWord(41, vm.Word{Op: op.GOTBL, Value: 500, ValueTwo: 0})
		m.SetWord(42, vm.Word{Op: op.GOTBL, Value: 501, ValueTwo: 1})
		m.SetWord(43, vm.Word{Op: op.GOTBL, Value: 502, ValueTwo: 2})
		for i := 0; i < 4 && m.PC <= 43; i++ {
			step(nil, nil)
		}
		if m.PC != tc.want {
			t.Errorf("%s: %d selects: want %d: got %d\n", opc, tc.choice, tc.want, m.PC)
		}
	}

	opc = op.GOEQ
	input = input_t{Cmp: vm.IS_LT}
	expect = expect_t{PC: 1, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{Cmp: vm.IS_EQ}
	expect = expect_t{PC: 8, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{Cmp: vm.IS_GR}
	expect = expect_t{PC: 1, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)

	opc = op.GOGE
	input = input_t{Cmp: vm.IS_LT}
	expect = expect_t{PC: 1, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{Cmp: vm.IS_EQ}
	expect = expect_t{PC: 8, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{Cmp: vm.IS_GR}
	expect = expect_t{PC: 8, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)

	opc = op.GOGR
	input = input_t{Cmp: vm.IS_LT}
	expect = expect_t{PC: 1, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{Cmp: vm.IS_EQ}
	expect = expect_t{PC: 1, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{Cmp: vm.IS_GR}
	expect = expect_t{PC: 8, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)

	opc = op.GOLE
	input = input_t{Cmp: vm.IS_LT}
	expect = expect_t{PC: 8, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{Cmp: vm.IS_EQ}
	expect = expect_t{PC: 8, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{Cmp: vm.IS_GR}
	expect = expect_t{PC: 1, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)

	opc = op.GOND
	input = input_t{C: '0'}
	expect = expect_t{PC: 1, A: 0, C: '0'}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{C: '9'}
	expect = expect_t{PC: 1, A: 9, C: '9'}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{C: 'A'}
	expect = expect_t{PC: 8, C: 'A'}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)

	opc = op.GONE
	input = input_t{Cmp: vm.IS_LT}
	expect = expect_t{PC: 8, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{Cmp: vm.IS_EQ}
	expect = expect_t{PC: 1, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{Cmp: vm.IS_GR}
	expect = expect_t{PC: 8, Cmp: input.Cmp}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)

	opc = op.GOPC
	input = input_t{C: '0'}
	expect = expect_t{PC: 1, C: '0'}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{C: 'A'}
	expect = expect_t{PC: 1, C: 'A'}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)
	input = input_t{C: '$'}
	expect = expect_t{PC: 8, C: '$'}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: 8})
	test(nil, nil)

	// GOSUB pushes the address of the word after it and branches. There is no
	// recursion in LOWL, so the stack is shallow, but it is a stack: a call
	// from inside a call has to come back through both.
	opc = op.GOSUB
	input = input_t{PC: 40, A: 3, B: 4, C: 5}
	expect = expect_t{PC: 500, A: input.A, B: input.B, C: input.C, RS: []int{41}}
	newvm()
	m.SetWord(input.PC, vm.Word{Op: opc, Value: 500})
	test(nil, nil)

	input = input_t{PC: 40, RS: []int{77}}
	expect = expect_t{PC: 500, RS: []int{77, 41}}
	newvm()
	m.SetWord(input.PC, vm.Word{Op: opc, Value: 500})
	test(nil, nil)

	// GOTBL is what an EXIT returns into, and what a GOADD selects with. It is
	// a GO that only happens when the number it carries is the one the last
	// EXIT or GOADD left behind, so a run of them is a jump table that is
	// walked until one of them matches.
	opc = op.GOTBL
	input = input_t{PC: 40, A: 3, B: 4, C: 5}
	expect = expect_t{PC: 600, A: input.A, B: input.B, C: input.C}
	newvm()
	m.Registers.JumpValue = 2
	m.SetWord(input.PC, vm.Word{Op: opc, Value: 600, ValueTwo: 2})
	test(nil, nil)

	expect = expect_t{PC: 41, A: input.A, B: input.B, C: input.C}
	newvm()
	m.Registers.JumpValue = 2
	m.SetWord(input.PC, vm.Word{Op: opc, Value: 600, ValueTwo: 1})
	test(nil, nil)

	// call and return, the way the two halves are actually written: a
	// subroutine with two exits, and a caller whose jump table is numbered
	// from one because exit 1 is the first entry.
	for _, tc := range []struct{ exit, want int }{
		{1, 600},
		{2, 601},
		{3, 43}, // more exits taken than the caller has entries: falls past both
	} {
		input = input_t{PC: 40}
		newvm()
		m.SetWord(40, vm.Word{Op: op.GOSUB, Value: 500})
		m.SetWord(41, vm.Word{Op: op.GOTBL, Value: 600, ValueTwo: 1})
		m.SetWord(42, vm.Word{Op: op.GOTBL, Value: 601, ValueTwo: 2})
		m.SetWord(500, vm.Word{Op: op.EXIT, Value: tc.exit})
		// call, return, then walk the table until an entry matches or the run
		// of them is over
		for i := 0; i < 6 && m.PC < 600 && m.PC != 43; i++ {
			step(nil, nil)
		}
		if m.PC != tc.want {
			t.Errorf("%s: exit %d: want %d: got %d\n", opc, tc.exit, tc.want, m.PC)
		}
		if len(m.RS) != 0 {
			t.Errorf("%s: exit %d: want an empty return stack: got %v\n", opc, tc.exit, m.RS)
		}
	}

	opc = op.HALT
	input = input_t{A: 3, B: 4, C: 5}
	expect = expect_t{PC: 0, A: input.A, B: input.B, C: input.C}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want halted: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrHalted) {
		t.Errorf("%s: want halted: got %v\n", opc, err)
	}
	if !m.Registers.Halted {
		t.Errorf("%s: want halted: got running\n", opc)
	}
	testA()
	testB()
	testC()
	testPC()
	testV()

	opc = op.IDENT
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want invalid op: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrInvalidOp) {
		t.Errorf("%s: want invalid op: got %v\n", opc, err)
	}

	opc = op.LAA
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}}
	expect = expect_t{PC: 1, A: input.V.address, B: input.B, C: input.C, V: input.V}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)

	opc = op.LAI
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}, V2: val_t{88, 837}}
	expect = expect_t{PC: 1, A: input.V2.value, B: input.B, C: input.C, V: input.V}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)

	opc = op.LAL
	input = input_t{A: 3, B: 4, C: 5, V: val_t{0, 88}}
	expect = expect_t{PC: 1, A: input.V.value, B: input.B, C: input.C, V: input.V}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.value})
	test(nil, nil)

	// LAM derive the pointer given by adding N-OF to the contents of B,
	//     and load A with the value pointed at by this (i.e. load A modified).
	opc = op.LAM
	input = input_t{A: 3, B: 84, C: 5, V: val_t{1, 837}}
	expect = expect_t{PC: 1, A: input.V.value, B: input.B, C: input.C, V: input.V}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: -83})
	test(nil, nil)

	opc = op.LAV
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}}
	expect = expect_t{PC: 1, A: input.V.value, B: input.B, C: input.C}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)

	opc = op.LBV
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}}
	expect = expect_t{PC: 1, A: input.A, B: input.V.value, C: input.C}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)

	opc = op.LCI
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}, V2: val_t{88, 837}}
	expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.V2.value, V: input.V}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)

	// LCM derive the pointer given by adding N-OF to the contents of B,
	//     and load C with the value pointed at by this (i.e. load C modified).
	opc = op.LCM
	input = input_t{A: 3, B: 84, C: 5, V: val_t{1, 837}}
	expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.V.value, V: input.V}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: -83})
	test(nil, nil)

	opc = op.LCN
	input = input_t{A: 3, B: 4, C: 5, V: val_t{0, 88}}
	expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.V.value, V: input.V}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.value})
	test(nil, nil)

	// MDERCH writes to the messages stream, not to the results stream, and it
	// writes the character in C as it stands: the dollar sign only means a
	// newline inside the text of a MESS statement.
	opc = op.MDERCH
	input = input_t{A: 3, B: 4, C: '$'}
	expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.C, Text: "$"}
	out = &bytes.Buffer{}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	test(nil, out)
	if b := out.Bytes(); len(b) != len(expect.Text) {
		t.Errorf("%s: out.len: want %d: got %d\n", opc, len(expect.Text), len(b))
	} else if s := string(b); s != expect.Text {
		t.Errorf("%s: out.text: want %q: got %q\n", opc, expect, s)
	}
	out = nil

	opc = op.MDLABEL
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want invalid op: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrInvalidOp) {
		t.Errorf("%s: want invalid op: got %v\n", opc, err)
	}

	opc = op.MDQUIT
	input = input_t{A: 3, B: 4, C: 5}
	expect = expect_t{PC: 0, A: input.A, B: input.B, C: input.C}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want quit: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrQuit) {
		t.Errorf("%s: want quit: got %v\n", opc, err)
	}
	if !m.Registers.Halted {
		t.Errorf("%s: want halted: got running\n", opc)
	}
	testA()
	testB()
	testC()
	testPC()
	testV()

	opc = op.MESS
	input = input_t{A: 3, B: 4, C: 5, Text: "$ABCDEFGHIJKLMNOPQRSTUVWXYZ$0123456789$.,;:()*/-+=\t \"$"}
	expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.C, Text: strings.ReplaceAll(input.Text, "$", "\n")}
	out = &bytes.Buffer{}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Text: input.Text})
	test(nil, out)
	if b := out.Bytes(); len(b) != len(expect.Text) {
		t.Errorf("%s: out.len: want %d: got %d\n", opc, len(expect.Text), len(b))
	} else if s := string(b); s != expect.Text {
		t.Errorf("%s: out.text: want %q: got %q\n", opc, expect, s)
	}
	out = nil

	opc = op.MULTL
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}}
	expect = expect_t{PC: 1, A: input.A * input.V.value, B: input.B, C: input.C}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.value})
	test(nil, nil)

	opc = op.NB
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want invalid op: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrInvalidOp) {
		t.Errorf("%s: want invalid op: got %v\n", opc, err)
	}

	opc = op.NCH
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want invalid op: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrInvalidOp) {
		t.Errorf("%s: want invalid op: got %v\n", opc, err)
	}

	opc = op.LINKB
	// LINKB returns from a linkroutine by branching to the address the
	// matching LINKR stored in LINKPT, leaving the subroutine stack alone.
	input = input_t{PC: 12, A: 3, B: 4, C: 5, RS: []int{7}, V: val_t{1, 44}}
	expect = expect_t{PC: input.V.value, A: input.A, B: input.B, C: input.C, RS: input.RS, V: input.V}
	newvm()
	m.SetWord(12, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)

	opc = op.LINKR
	// LINKR moves the return address the GOSUB pushed off the subroutine
	// stack and into LINKPT, so that the LINKB can find it.
	input = input_t{A: 3, B: 4, C: 5, RS: []int{7, 19}, V: val_t{address: 1}}
	expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.C, RS: []int{7}, V: val_t{1, 19}}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)
	// and it must complain rather than pop an empty stack
	input = input_t{V: val_t{address: 1}}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	if err := m.Step(nil, nil); !errors.Is(err, vm.ErrStackUnderflow) {
		t.Errorf("%s: want stack underflow: got %v\n", opc, err)
	}

	opc = op.NOOP
	input = input_t{A: 3, B: 4, C: 5}
	expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.C}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.value})
	test(nil, nil)

	opc = op.ORL
	input = input_t{A: 3, B: 12, C: 49, V: val_t{1, 12}}
	expect = expect_t{PC: 1, A: input.A | input.V.value, B: input.B, C: input.C, V: input.V}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.value})
	test(nil, nil)

	opc = op.PRGEN
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want invalid op: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrInvalidOp) {
		t.Errorf("%s: want invalid op: got %v\n", opc, err)
	}

	opc = op.PRGST
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want invalid op: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrInvalidOp) {
		t.Errorf("%s: want invalid op: got %v\n", opc, err)
	}

	opc = op.SAL
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}}
	expect = expect_t{PC: 1, A: input.A - input.V.value, B: input.B, C: input.C}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.value})
	test(nil, nil)

	opc = op.SAV
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}}
	expect = expect_t{PC: 1, A: input.A - input.V.value, B: input.B, C: input.C, V: input.V}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)

	opc = op.SBL
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}}
	expect = expect_t{PC: 1, A: input.A, B: input.B - input.V.value, C: input.C}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.value})
	test(nil, nil)

	opc = op.SBV
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}}
	expect = expect_t{PC: 1, A: input.A, B: input.B - input.V.value, C: input.C, V: val_t{1, input.V.value}}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)

	opc = op.STI
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 21}, V2: val_t{21, 144}}
	expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.C, V: input.V, V2: val_t{input.V2.address, input.A}}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)

	opc = op.STR
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want invalid op: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrInvalidOp) {
		t.Errorf("%s: want invalid op: got %v\n", opc, err)
	}

	opc = op.STV
	input = input_t{A: 3, B: 4, C: 5, V: val_t{1, 88}}
	expect = expect_t{PC: 1, A: input.A, B: input.B, C: input.C, V: val_t{1, input.A}}
	newvm()
	m.SetWord(0, vm.Word{Op: opc, Value: input.V.address})
	test(nil, nil)

	opc = op.SUBR
	input = input_t{}
	expect = expect_t{PC: 1}
	newvm()
	m.SetWord(0, vm.Word{Op: opc})
	if err := m.Step(nil, nil); err == nil {
		t.Errorf("%s: want invalid op: got nil\n", opc)
	} else if !errors.Is(err, vm.ErrInvalidOp) {
		t.Errorf("%s: want invalid op: got %v\n", opc, err)
	}

	// UNSTK takes the number at the top of the backwards stack and puts it in
	// a variable. A is clobbered on the way, because the manual's mapping
	// loads it and stores it back out again.
	opc = op.UNSTK
	input = input_t{PC: 40, A: 3, B: 4, C: 5}
	expect = expect_t{PC: 41, A: 777, B: input.B, C: input.C, V: val_t{30, 777}}
	stackvm(200, 64)
	m.SetWord(input.PC, vm.Word{Op: opc, Value: 30})
	m.Core[m.Registers.LFPT].Value = 260
	m.SetWord(260, vm.Word{Op: op.CON, Value: 777})
	test(nil, nil)
	if lfpt() != 261 {
		t.Errorf("%s: lfpt: want 261: got %d\n", opc, lfpt())
	}

	// the round trip: what BSTK put on comes back off, in the order a stack
	// gives it back, and LFPT ends where it started.
	input = input_t{PC: 40, A: 111}
	stackvm(200, 64)
	m.SetWord(40, vm.Word{Op: op.BSTK})
	m.SetWord(41, vm.Word{Op: opc, Value: 30})
	m.PC = 40
	step(nil, nil)
	m.A, m.PC = 222, 40
	step(nil, nil)
	if lfpt() != 262 {
		t.Errorf("%s: lfpt after two BSTK: want 262: got %d\n", opc, lfpt())
	}
	m.PC = 41
	step(nil, nil)
	testWord("first off", 30, 222)
	m.PC = 41
	step(nil, nil)
	testWord("second off", 30, 111)
	if lfpt() != 264 {
		t.Errorf("%s: lfpt after unstacking both: want 264: got %d\n", opc, lfpt())
	}

	// HASH, THASH, WTHS and RL are table items rather than instructions. The
	// assembler fills in their values and nothing should ever branch to one,
	// so stepping onto one has to be reported rather than quietly skipped.
	for _, opc = range []op.Code{op.HASH, op.THASH, op.WTHS, op.RL} {
		input = input_t{}
		newvm()
		m.SetWord(0, vm.Word{Op: opc})
		if err := m.Step(nil, nil); !errors.Is(err, vm.ErrNotExecutable) {
			t.Errorf("%s: want not executable: got %v\n", opc, err)
		}
	}

	// MDCONV, MDFIND, MDOP, MDOUCH and MDREAD are the machine-dependent
	// subroutines. They need a host and a program's variables around them, so
	// they are tested in md_test.go rather than a word at a time here.
}
