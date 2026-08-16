// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm_test

import (
	"errors"
	"io"
	"testing"

	"github.com/mdhender/maclo/pkg/lowl/op"
	"github.com/mdhender/maclo/pkg/lowl/vm"
)

// testHost stands in for the outside world: a fixed source text, and two
// buffers for the two output streams.
type testHost struct {
	source   []int
	next     int
	results  []int
	messages []int
}

func (h *testHost) ReadChar() (int, error) {
	if h.next >= len(h.source) {
		return 0, io.EOF
	}
	ch := h.source[h.next]
	h.next++
	return ch, nil
}

func (h *testHost) WriteChar(ch int) error {
	h.results = append(h.results, ch)
	return nil
}

func (h *testHost) WriteMessage(ch int) error {
	h.messages = append(h.messages, ch)
	return nil
}

func (h *testHost) resultText() string {
	return text(h.results)
}

func (h *testHost) messageText() string {
	return text(h.messages)
}

func text(chars []int) string {
	b := make([]byte, len(chars))
	for i, ch := range chars {
		b[i] = byte(ch)
	}
	return string(b)
}

// mdvm returns a machine holding one machine-dependent instruction, with the
// variables that instruction reads declared at addresses of our choosing.
// The names are the ones the ML/I source declares; where they are is up to
// whoever assembled it, which is why the machine looks them up.
func mdvm(t *testing.T, opc op.Code, variables map[string]int) *vm.VM {
	t.Helper()
	m := vm.New()
	m.Registers.Start = 1_000
	m.Registers.Last = 2_000
	for name, address := range variables {
		m.Symbols[name] = address
	}
	m.PC = m.Registers.Start
	m.SetWord(m.PC, vm.Word{Op: opc})
	return m
}

// TestMDCONV checks the decimal text MDCONV hands back through IDPT and
// IDLEN. It is the one MD subroutine that has to find storage of its own: the
// caller reads the digits through a pointer, so they have to outlive the call.
func TestMDCONV(t *testing.T) {
	for _, tc := range []struct {
		meval int
		want  string
	}{
		{0, "0"},
		{7, "7"},
		{-407, "-407"},
		{2147483647, "2147483647"},
		{-2147483647, "-2147483647"},
	} {
		m := mdvm(t, op.MDCONV, map[string]int{"MEVAL": 100, "IDPT": 101, "IDLEN": 102})
		m.SetWord(100, vm.Word{Op: op.CON, Value: tc.meval})

		if err := m.Step(nil, nil); err != nil {
			t.Fatalf("mdconv(%d): %v", tc.meval, err)
		}

		idpt, idlen := m.Core[101].Value, m.Core[102].Value
		if want := len(tc.want) * m.Registers.LCH; idlen != want {
			t.Errorf("mdconv(%d): idlen: want %d: got %d", tc.meval, want, idlen)
		}
		var got []int
		for i := 0; i < idlen; i += m.Registers.LCH {
			got = append(got, m.Core[idpt+i].Value)
		}
		if s := text(got); s != tc.want {
			t.Errorf("mdconv(%d): want %q: got %q", tc.meval, tc.want, s)
		}
		// the digits live in the words reserved below the program, where
		// neither the assembled logic nor the workspace can reach them.
		if idpt >= m.Registers.Start {
			t.Errorf("mdconv(%d): idpt %d is not below the program", tc.meval, idpt)
		}
	}
}

// TestMDFINDAgreesWithTheAssembler is the point of the whole exercise: a name
// the assembler put on a chain has to hash to that same chain when MDFIND
// looks it up, or a construction that is present will not be found.
func TestMDFINDAgreesWithTheAssembler(t *testing.T) {
	const hashpt, htabpt, idpt, idlen, spt = 100, 101, 102, 103, 104
	const table, atom = 500, 600

	for _, name := range []string{"MCDEF", "MCINS", "MCSKIP", "MCSET", "MCGO", "A"} {
		m := mdvm(t, op.MDFIND, map[string]int{
			"HASHPT": hashpt, "HTABPT": htabpt, "IDPT": idpt, "IDLEN": idlen, "SPT": spt,
		})
		// lay the atom out in core the way the scanner leaves it: IDPT at the
		// first character, SPT at the last, IDLEN the length in characters
		// times LCH.
		for i := 0; i < len(name); i++ {
			m.SetWord(atom+i*m.Registers.LCH, vm.Word{Op: op.CON, Value: int(name[i])})
		}
		m.SetWord(hashpt, vm.Word{Op: op.CON, Value: table})
		m.SetWord(idpt, vm.Word{Op: op.CON, Value: atom})
		m.SetWord(spt, vm.Word{Op: op.CON, Value: atom + (len(name)-1)*m.Registers.LCH})
		m.SetWord(idlen, vm.Word{Op: op.CON, Value: len(name) * m.Registers.LCH})

		if err := m.Step(nil, nil); err != nil {
			t.Fatalf("mdfind(%s): %v", name, err)
		}

		want := table + vm.HashName(name, m.Registers.LCH, m.Registers.LNM)
		if got := m.Core[htabpt].Value; got != want {
			t.Errorf("mdfind(%s): htabpt: want %d: got %d", name, want, got)
		}
	}
}

// TestMDOP covers both operations and both exits.
//
// Division is the trap. Chapter 2 of the user's manual asks for the greatest
// integer that does not exceed the exact answer, and gives -5/4 and 5/-4 as
// -2; Go's own division would answer -1 to both.
func TestMDOP(t *testing.T) {
	const meval, op1, opsw = 100, 101, 102
	const multiply, divide = 1, 0

	for _, tc := range []struct {
		name       string
		sw         int
		op1, meval int
		want       int
		exit       int
	}{
		{"multiply", multiply, 6, 7, 42, 2},
		{"multiply by zero", multiply, 6, 0, 0, 2},
		{"divide", divide, 7, 6, 1, 2},
		{"divide exactly", divide, 42, 7, 6, 2},
		{"divide truncating down", divide, -5, 4, -2, 2},
		{"divide a negative divisor", divide, 5, -4, -2, 2},
		{"divide two negatives", divide, -6, -4, 1, 2},
		{"divide by zero", divide, 6, 0, 0, 1},
	} {
		m := mdvm(t, op.MDOP, map[string]int{"MEVAL": meval, "OP1": op1, "OPSW": opsw})
		m.SetWord(meval, vm.Word{Op: op.CON, Value: tc.meval})
		m.SetWord(op1, vm.Word{Op: op.CON, Value: tc.op1})
		m.SetWord(opsw, vm.Word{Op: op.CON, Value: tc.sw})

		if err := m.Step(nil, nil); err != nil {
			t.Fatalf("mdop(%s): %v", tc.name, err)
		}

		if got := m.Registers.JumpValue; got != tc.exit {
			t.Errorf("mdop(%s): exit: want %d: got %d", tc.name, tc.exit, got)
		}
		if tc.exit == 1 {
			// exit 1 is taken without any further processing, so MEVAL is
			// still the divisor the caller put there.
			if got := m.Core[meval].Value; got != tc.meval {
				t.Errorf("mdop(%s): meval: want it untouched (%d): got %d", tc.name, tc.meval, got)
			}
			continue
		}
		if got := m.Core[meval].Value; got != tc.want {
			t.Errorf("mdop(%s): meval: want %d: got %d", tc.name, tc.want, got)
		}
	}
}

// TestMDREAD checks both exits. Exit 1 is the end of the source text and exit
// 2 is a character, and getting them the wrong way round would have ML/I read
// on past a character that was never there.
func TestMDREAD(t *testing.T) {
	host := &testHost{source: []int{'h', 'i', '\n'}}
	m := mdvm(t, op.MDREAD, nil)
	m.Host = host

	for _, want := range host.source {
		m.PC = m.Registers.Start
		if err := m.Step(nil, nil); err != nil {
			t.Fatalf("mdread: %v", err)
		}
		if m.C != want {
			t.Errorf("mdread: c: want %q: got %q", rune(want), rune(m.C))
		}
		if got := m.Registers.JumpValue; got != 2 {
			t.Errorf("mdread: exit: want 2: got %d", got)
		}
	}

	m.PC = m.Registers.Start
	if err := m.Step(nil, nil); err != nil {
		t.Fatalf("mdread: %v", err)
	}
	if got := m.Registers.JumpValue; got != 1 {
		t.Errorf("mdread at end of source: exit: want 1: got %d", got)
	}
}

// TestMDREADWithoutAHost checks that a machine with no input says so. A read
// that quietly returned nothing would look exactly like an empty source text.
func TestMDREADWithoutAHost(t *testing.T) {
	m := mdvm(t, op.MDREAD, nil)
	if err := m.Step(nil, nil); !errors.Is(err, vm.ErrNoHost) {
		t.Errorf("mdread: want ErrNoHost: got %v", err)
	}
}

// TestTheTwoOutputStreamsStaySeparate pins which stream each of the three
// output statements uses. MESS and MDERCH write messages and MDOUCH writes
// results; sending MESS to the results stream would put the end of process
// report in with the output text.
func TestTheTwoOutputStreamsStaySeparate(t *testing.T) {
	host := &testHost{}
	m := vm.New()
	m.Host = host
	m.Registers.Start = 1_000
	m.Registers.Last = 2_000
	m.PC = m.Registers.Start
	m.C = 'x'
	m.SetWord(1_000, vm.Word{Op: op.MDOUCH})
	m.SetWord(1_001, vm.Word{Op: op.MDERCH})
	m.SetWord(1_002, vm.Word{Op: op.MESS, Text: "one$two"})
	m.SetWord(1_003, vm.Word{Op: op.MDQUIT})

	if err := m.Run(nil, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, want := host.resultText(), "x"; got != want {
		t.Errorf("results: want %q: got %q", want, got)
	}
	if got, want := host.messageText(), "xone\ntwo"; got != want {
		t.Errorf("messages: want %q: got %q", want, got)
	}
}
