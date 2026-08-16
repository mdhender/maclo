// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/maloquacious/ml_i/pkg/lowl/op"
)

// The machine dependent subroutines, as Supplement 3 specifies them.
//
// A GOSUB to one of these assembles into a single instruction rather than a
// call, so there is no return address to pop: the word after the instruction
// is already the first word of the caller's jump table. A subroutine with
// more than one exit therefore does what EXIT does, and sets the jump value
// that the GOTBL words compare against. Exit numbers are one based.
//
// MDCONV, MDFIND and MDOP are here rather than in the Host interface because
// none of them reaches outside the machine. They read and write the program's
// own variables, and the machine can find those through Symbols.

// svPseudoAlpha is S6, which holds the code of a character that is not a letter
// or a digit but is to be treated as though it were one, so that it can appear
// inside an atom. Chapter 2 of the user's manual gives the underscore of
// CURRENT_POSITION as the example.
const svPseudoAlpha = 6

// pseudoAlpha returns the character code S6 holds, or -1 when there is none.
//
// This is the machine's business rather than the program's. The MI-logic never
// reads S6 — it splits text into atoms with GOPC and nothing else — so the only
// place the extra character can be honoured is in GOPC itself, which is why
// ispunct is a method. A program with no S-variables, such as LOWLTEST, gets
// the kernel's plain definition.
//
// The value is read on every call because a macro may assign to S6 between any
// two characters, and the answer has to change with it.
func (m *VM) pseudoAlpha() int {
	if m.Registers.SVARPT == 0 {
		return -1
	}
	return m.directLoad(m.directLoad(m.Registers.SVARPT) - svPseudoAlpha*m.Registers.LNM)
}

// stackOverflow hands control to the program's own storage overflow handler.
//
// The mappings the kernel manual gives for FSTK, BSTK and CFSTK all end in
// "GOGE ERLSO", and it says of that label that it "is the label of the code
// that deals with stack overflow; it is present in every MI-logic". So running
// out of storage is not a fault the machine reports and stops on. It is a
// branch, and what it branches to is the program's own diagnostic: in ML/I,
// ERLSO prints the message, counts the error in S5, prints the context that
// says where the storage went, and ends through the normal finalisation.
// Stopping the machine here instead throws all of that away and leaves a
// debugging stream with nothing on it.
//
// A program with no ERLSO has nowhere to go — LOWLTEST is one — so for that
// one the machine does report and stop.
func (m *VM) stackOverflow(code op.Code) error {
	if erlso, ok := m.Symbols["ERLSO"]; ok {
		m.PC = erlso
		return nil
	}
	return fmt.Errorf("%d: %s: %w", m.PC-1, code, ErrStackOverflow)
}

// variable returns the address the program's DCL gave name.
func (m *VM) variable(name string) (int, error) {
	address, ok := m.Symbols[name]
	if !ok {
		return 0, fmt.Errorf("%s: %w", name, ErrNoSymbol)
	}
	return address, nil
}

// mdconv converts MEVAL to decimal text, points IDPT at the first character
// and sets IDLEN to the length. One exit.
//
// The text is built in the words New reserved for it, because IDPT is a
// pointer the rest of ML/I will read characters through and the answer has to
// outlive the call.
func (m *VM) mdconv() error {
	meval, err := m.variable("MEVAL")
	if err != nil {
		return err
	}
	idpt, err := m.variable("IDPT")
	if err != nil {
		return err
	}
	idlen, err := m.variable("IDLEN")
	if err != nil {
		return err
	}

	// Itoa gives exactly what is asked for: decimal digits, a leading minus
	// when the value is negative, and no redundant leading zeros.
	text := strconv.Itoa(m.directLoad(meval))
	if len(text) > MaxNumberText {
		return fmt.Errorf("MDCONV: %q: %w", text, ErrNumberTooLong)
	}
	for i := 0; i < len(text); i++ {
		m.directStore(m.Registers.NUMPT+i*m.Registers.LCH, int(text[i]))
	}
	m.directStore(idpt, m.Registers.NUMPT)
	m.directStore(idlen, len(text)*m.Registers.LCH)
	return nil
}

// mdfind sets HTABPT to the head of the hash chain the atom described by
// IDPT, SPT and IDLEN belongs on. One exit.
//
// The chain heads were laid out by the assembler, from the THASH statement,
// and the program has already pointed HASHPT at them. Both ends of that
// arrangement have to agree on which chain a name lands on, so the arithmetic
// is not repeated here: it is hashAtom, the same function the assembler used.
func (m *VM) mdfind() error {
	hashpt, err := m.variable("HASHPT")
	if err != nil {
		return err
	}
	htabpt, err := m.variable("HTABPT")
	if err != nil {
		return err
	}
	idpt, err := m.variable("IDPT")
	if err != nil {
		return err
	}
	idlen, err := m.variable("IDLEN")
	if err != nil {
		return err
	}
	spt, err := m.variable("SPT")
	if err != nil {
		return err
	}

	// IDPT points at the first character of the atom and SPT at its last.
	first := m.indirectLoad(idpt)
	last := m.indirectLoad(spt)
	chain := hashAtom(m.directLoad(idlen), first, last, m.Registers.LNM)
	m.directStore(htabpt, m.directLoad(hashpt)+chain)
	return nil
}

// mdop multiplies or divides OP1 by MEVAL and leaves the answer in MEVAL.
// Two exits: exit 1 for a division by zero, exit 2 for an answer.
//
// OPSW says which operation is wanted: one means multiply, anything else
// means divide.
func (m *VM) mdop() error {
	meval, err := m.variable("MEVAL")
	if err != nil {
		return err
	}
	op1, err := m.variable("OP1")
	if err != nil {
		return err
	}
	opsw, err := m.variable("OPSW")
	if err != nil {
		return err
	}

	left, right := m.directLoad(op1), m.directLoad(meval)
	if m.directLoad(opsw) == 1 {
		m.directStore(meval, left*right)
		m.Registers.JumpValue = 2
		return nil
	}
	if right == 0 {
		// the divisor is tested before anything else happens, and this exit
		// is taken without any further processing.
		m.Registers.JumpValue = 1
		return nil
	}
	m.directStore(meval, floorDiv(left, right))
	m.Registers.JumpValue = 2
	return nil
}

// floorDiv is division as chapter 2 of the user's manual defines it: the
// result is the greatest integer that does not exceed the exact answer. That
// is not Go's division, which truncates towards zero, and the difference
// shows on every negative operand: the manual gives -5/4 and 5/-4 as -2,
// where Go gives -1.
func floorDiv(a, b int) int {
	quotient := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		quotient--
	}
	return quotient
}

// mdread reads one character of the source text into C. Two exits: exit 1
// when the source is exhausted, exit 2 when a character was read.
func (m *VM) mdread(host Host) error {
	ch, err := host.ReadChar()
	if errors.Is(err, io.EOF) {
		m.Registers.JumpValue = 1
		return nil
	} else if err != nil {
		return fmt.Errorf("MDREAD: %w", err)
	}
	m.C = ch
	m.Registers.JumpValue = 2
	return nil
}

// mdouch writes C to the results stream. One exit.
func (m *VM) mdouch(host Host) error {
	if err := host.WriteChar(m.C); err != nil {
		return fmt.Errorf("MDOUCH: %w", err)
	}
	return nil
}

// mderch writes C to the messages stream. One exit.
func (m *VM) mderch(host Host) error {
	if err := host.WriteMessage(m.C); err != nil {
		return fmt.Errorf("MDERCH: %w", err)
	}
	return nil
}

// mess writes the text of a MESS statement to the messages stream, a dollar
// sign standing for a newline.
//
// It goes through the host one character at a time, and not straight to a
// writer, because the messages stream is metered: ML/I keeps a quota of lines
// in S12 and the host is what counts them.
func (m *VM) mess(host Host, text string) error {
	for i := 0; i < len(text); i++ {
		ch := int(text[i])
		if ch == '$' {
			ch = '\n'
		}
		if err := host.WriteMessage(ch); err != nil {
			return fmt.Errorf("MESS: %w", err)
		}
	}
	return nil
}
