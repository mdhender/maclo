// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm

import (
	"fmt"
	"github.com/maloquacious/ml_i/pkg/lowl/op"
	"io"
)

func (m *VM) Step(stdout, stderr io.Writer) error {
	if m.Registers.Halted {
		return ErrHalted
	}

	w := m.Core[m.PC]
	m.PC = m.PC + 1

	memcpy := func(src, dst, length int) {
		// SRCPT points at the start of the source field.
		// DSTPT points to the start of the destination field.
		// Register A contains the length of the field (number of words to move)
		tmp := make([]Word, length, length)
		for offset := 0; offset < length; offset++ {
			tmp[offset] = m.Core[src+offset]
		}
		for offset := 0; offset < length; offset++ {
			m.Core[dst+offset] = tmp[offset]
		}
	}

	switch w.Op {
	case op.AAL: // add a literal value to register A
		literalValue := w.Value
		m.A = m.A + literalValue
	case op.AAV: // add a variable to register A
		variableAddress := w.Value
		variableValue := m.directLoad(variableAddress)
		m.A = m.A + variableValue
	case op.ABV: // add a variable to register B
		variableAddress := w.Value
		variableValue := m.directLoad(variableAddress)
		m.B = m.B + variableValue
	case op.ANDL: // bitwise "AND" a literal value with register A
		literalValue := w.Value
		m.A = m.A & literalValue
	case op.ANDV: // bitwise AND a variable with register A
		variableAddress := w.Value
		variableValue := m.directLoad(variableAddress)
		m.A = m.A & variableValue
	case op.BMOVE: // backwards block move
		// SRCPT points at the start of the source field.
		// DSTPT points to the start of the destination field.
		// Register A contains the length of the field (number of words to move)
		memcpy(m.directLoad(w.Value), m.directLoad(w.ValueTwo), m.A)
	case op.BSTK: // stack A on backwards stack
		// preserve A
		a := m.A

		// LAV   LFPT     // load A with value of LFPT
		variableAddress := m.Registers.LFPT
		variableValue := m.directLoad(variableAddress)
		m.A = variableValue

		// SAL  OF(LNM)  // subtract LNM from register A
		literalValue := m.Registers.LNM
		m.A = m.A - literalValue

		// STV  LFPT     // store register A in LFPT
		variableAddress = m.Registers.LFPT
		variableValue = m.A
		m.directStore(variableAddress, variableValue)

		// restore A
		m.A = a

		// STI   LFPT     // store A in address pointed at by LFPT
		valueToStore := m.A
		variableAddress = m.Registers.LFPT
		m.indirectStore(variableAddress, valueToStore)

		// LAV   FFPT     // load A with value of FFPT
		variableAddress = m.Registers.FFPT
		variableValue = m.directLoad(variableAddress)
		m.A = variableValue

		// CAV   LFPT     // compare A with the value of LFPT
		variableAddress = m.Registers.LFPT
		variableValue = m.directLoad(variableAddress)
		m.compare(m.A, variableValue)

		// GOGE  ERLSO    // if EQ or GT, the stacks have met
		if m.Registers.Cmp == IS_EQ || m.Registers.Cmp == IS_GR {
			return m.stackOverflow(w.Op)
		}
	case op.BUMP: // increase a variable by a literal value
		literalValue := w.ValueTwo
		variableAddress := w.Value
		variableValue := m.directLoad(variableAddress)
		m.directStore(variableAddress, literalValue+variableValue)
	case op.CAI: // compare contents of address pointed to by V to register A
		variableAddress := w.Value
		indirectValue := m.indirectLoad(variableAddress)
		m.compare(m.A, indirectValue)
	case op.CAL: // compare register A with a literal value
		literalValue := w.Value
		m.compare(m.A, literalValue)
	case op.CAV: // compare A with the value of variable
		variableAddress := w.Value
		variableValue := m.directLoad(variableAddress)
		m.compare(m.A, variableValue)
	case op.CCI: // compare contents of address pointed to by V to register C
		variableAddress := w.Value
		indirectValue := m.indirectLoad(variableAddress)
		m.compare(m.C, indirectValue)
	case op.CCL: // compare register C with a literal value
		m.compare(m.C, w.Value)
	case op.CCN: // compare register C with named character
		literalValue := w.Value
		m.compare(m.C, literalValue)
	case op.CFSTK: // stack C on forwards stack
		// CSTK is implemented as FSTK except C is stored and FFPT is incremented by OF(LCH)
		// STI   FFPT     // store A in address pointed at by FFPT
		valueToStore := m.C
		variableAddress := m.Registers.FFPT
		m.indirectStore(variableAddress, valueToStore)

		// LAV   FFPT     // load A with value of FFPT
		variableAddress = m.Registers.FFPT
		variableValue := m.directLoad(variableAddress)
		m.A = variableValue

		// AAL   OF(LCH)  // add LCH to register A
		//
		// LCH, not LNM: this is the one place the two differ, and on a machine
		// where a character and a number are the same size the difference is
		// invisible.
		literalValue := m.Registers.LCH
		m.A = m.A + literalValue

		// STV   FFPT     // store register A in FFPT
		variableAddress = m.Registers.FFPT
		variableValue = m.A
		m.directStore(variableAddress, variableValue)

		// CAV   LFPT     // compare A with the value of LFPT
		variableAddress = m.Registers.LFPT
		variableValue = m.directLoad(variableAddress)
		m.compare(m.A, variableValue)

		// GOGE  ERLSO    // if EQ or GT, the stacks have met
		if m.Registers.Cmp == IS_EQ || m.Registers.Cmp == IS_GR {
			return m.stackOverflow(w.Op)
		}
	case op.CLEAR: // set variable to zero
		variableAddress := w.Value
		m.directStore(variableAddress, 0)
	case op.CSS: // clear the subroutine stack
		// CSS marks a label in the main logic that is branched to from inside
		// a subroutine. That branch can come from any depth, so this empties
		// the stack rather than popping one link: the main logic runs with no
		// return address outstanding. An empty stack is not an error here,
		// because the same label is also reached by falling into it.
		m.RS = m.RS[:0]
	case op.EXIT: // exit from subroutine
		// An EXIT with nothing to return to is a fault in the program — a
		// subroutine fallen into rather than called, or one whose link a CSS
		// has already cleared. Reporting it is the same choice LINKR makes,
		// and it is better than indexing off the end of the stack.
		if len(m.RS) == 0 {
			return fmt.Errorf("%d: %s: %w", m.PC-1, w.Op, ErrStackUnderflow)
		}
		// pop the return address from the stack
		m.PC, m.RS = m.RS[len(m.RS)-1], m.RS[:len(m.RS)-1]
		// update the test register used by GOADD and GOBRPC
		m.Registers.JumpValue = w.Value
	case op.FMOVE: // forwards block move
		// SRCPT points at the start of the source field.
		// DSTPT points to the start of the destination field.
		// Register A contains the length of the field (number of words to move)
		memcpy(m.directLoad(w.Value), m.directLoad(w.ValueTwo), m.A)
	case op.FSTK: // stack A on forwards stack
		// STI   FFPT     // store A in address pointed at by FFPT
		valueToStore := m.A
		variableAddress := m.Registers.FFPT
		m.indirectStore(variableAddress, valueToStore)

		// LAV   FFPT     // load A with value of FFPT
		variableAddress = m.Registers.FFPT
		variableValue := m.directLoad(variableAddress)
		m.A = variableValue

		// AAL   OF(LNM)  // add LNM to register A
		literalValue := m.Registers.LNM
		m.A = m.A + literalValue

		// STV   FFPT     // store register A in FFPT
		variableAddress = m.Registers.FFPT
		variableValue = m.A
		m.directStore(variableAddress, variableValue)

		// CAV   LFPT     // compare A with the value of LFPT
		variableAddress = m.Registers.LFPT
		variableValue = m.directLoad(variableAddress)
		m.compare(m.A, variableValue)

		// GOGE  ERLSO    // if EQ or GT, the stacks have met
		if m.Registers.Cmp == IS_EQ || m.Registers.Cmp == IS_GR {
			return m.stackOverflow(w.Op)
		}
	case op.GO: // unconditional branch
		m.PC = w.Value
	case op.GOADD: // multi-way branch
		m.Registers.JumpValue = m.directLoad(w.Value)
	case op.GOEQ: // branch if equal
		if m.Registers.Cmp == IS_EQ {
			m.PC = w.Value
		}
	case op.GOGE: // branch if greater than or equal
		if m.Registers.Cmp == IS_GR || m.Registers.Cmp == IS_EQ {
			m.PC = w.Value
		}
	case op.GOGR: // branch if greater than
		if m.Registers.Cmp == IS_GR {
			m.PC = w.Value
		}
	case op.GOLE: // branch if less than or equal
		if m.Registers.Cmp == IS_LT || m.Registers.Cmp == IS_EQ {
			m.PC = w.Value
		}
	case op.GOLT: // branch if less than
		if m.Registers.Cmp == IS_LT {
			m.PC = w.Value
		}
	case op.GOND: // branch if C is not a digit; otherwise put value in A
		if !isdigit(m.C) {
			m.PC = w.Value
		} else {
			m.A = m.C - '0'
		}
	case op.GONE: // branch if not equal
		if m.Registers.Cmp != IS_EQ {
			m.PC = w.Value
		}
	case op.GOPC: // branch if C is a punctuation character
		if m.ispunct(m.C) {
			m.PC = w.Value
		}
	case op.GOSUB: // call subroutine
		// push return address on to the return stack
		m.RS = append(m.RS, m.PC)
		// go to the subroutine
		m.PC = w.Value
	case op.HASH, op.THASH, op.WTHS, op.RL: // table items, not instructions
		// The assembler lays these out in the data sections and fills in their
		// values. Reaching one means control has run off the end of the logic
		// into the tables, which is a bug worth reporting rather than a word
		// worth executing.
		return fmt.Errorf("%d: %s: %w", m.PC-1, w.Op, ErrNotExecutable)
	case op.LINKB: // return from a linkroutine
		// the matching LINKR left the return address in LINKPT.
		variableAddress := w.Value
		m.PC = m.directLoad(variableAddress)
	case op.LINKR: // enter a linkroutine
		// A linkroutine returns through LINKPT rather than through the
		// subroutine stack, so move the return address the GOSUB pushed into
		// LINKPT. STKARG is recursive and saves LINKPT itself before it calls
		// anything that could overwrite it.
		if len(m.RS) == 0 {
			return fmt.Errorf("%d: %s: %w", m.PC-1, w.Op, ErrStackUnderflow)
		}
		returnAddress := m.RS[len(m.RS)-1]
		m.RS = m.RS[:len(m.RS)-1]
		variableAddress := w.Value
		m.directStore(variableAddress, returnAddress)
	case op.ORL: // bitwise inclusive "OR" a literal value with register A
		literalValue := w.Value
		m.A = m.A | literalValue
	case op.GOTBL: // jump table for exit instructions
		if w.ValueTwo == m.Registers.JumpValue {
			m.PC = w.Value
		}
	case op.HALT: // halt the machine
		// force the program counter back to this instruction
		m.PC = m.PC - 1
		// signal that we have halted
		m.Registers.Halted = true
		return ErrHalted
	case op.LAA: // load address of variable V into register A
		variableAddress := w.Value
		m.A = variableAddress
	case op.LAI: // load A with contents of the address pointed to by variable V
		variableAddress := w.Value
		indirectValue := m.indirectLoad(variableAddress)
		m.A = indirectValue
	case op.LAL: // load literal value into register A
		literalValue := w.Value
		m.A = literalValue
	case op.LAM: // load contents of address pointed to by register B + N-OF into register A
		literalValue := w.Value
		indexedValue := m.indexedLoad(literalValue)
		m.A = indexedValue
	case op.LAV: // load A with value of variable V
		variableAddress := w.Value
		variableValue := m.directLoad(variableAddress)
		m.A = variableValue
	case op.LBV: // load B with value of variable B
		variableAddress := w.Value
		variableValue := m.directLoad(variableAddress)
		m.B = variableValue
	case op.LCI: // load C with contents of the address pointed to by variable V
		variableAddress := w.Value
		indirectValue := m.indirectLoad(variableAddress)
		m.C = indirectValue
	case op.LCM: // load contents of address pointed to by register B + N-OF into register A
		literalValue := w.Value
		indexedValue := m.indexedLoad(literalValue)
		m.C = indexedValue
	case op.LCN: // load C with named character
		literalValue := w.Value
		m.C = literalValue
	case op.MDERCH: // copy register C to the messages stream
		return m.mderch(m.host(stdout, stderr))
	case op.MDQUIT: // graceful exit requested
		// force the program counter back to this instruction
		m.PC = m.PC - 1
		// signal that we have stopped the machine
		m.Registers.Halted = true
		return ErrQuit
	case op.MESS: // copy text to the messages stream
		return m.mess(m.host(stdout, stderr), w.Text)
	case op.MULTL: // multiply register A by a literal value
		literalValue := w.Value
		m.A = m.A * literalValue
	case op.NOOP: // noop
		// do nothing
	case op.SAL: // subtract a literal value from register A
		literalValue := w.Value
		m.A = m.A - literalValue
	case op.SAV: // subtract a variable from register A
		variableAddress := w.Value
		variableValue := m.directLoad(variableAddress)
		m.A = m.A - variableValue
	case op.SBL: // subtract a literal value from register B
		literalValue := w.Value
		m.B = m.B - literalValue
	case op.SBV: // subtract a variable from register B
		variableAddress := w.Value
		variableValue := m.directLoad(variableAddress)
		m.B = m.B - variableValue
	case op.STI: // store register A in address pointed at by variable V
		valueToStore := m.A
		variableAddress := w.Value
		m.indirectStore(variableAddress, valueToStore)
	case op.STV: // store register A in variable
		variableAddress := w.Value
		variableValue := m.A
		m.directStore(variableAddress, variableValue)
	case op.UNSTK: // unstack from backwards stack
		parmVariableAddress := w.Value

		// LAI  LFPT         // load A with contents of the address pointed to by variable V
		variableAddress := m.Registers.LFPT
		indirectValue := m.indirectLoad(variableAddress)
		m.A = indirectValue

		// STV  V             // store register A in variable
		variableAddress = parmVariableAddress
		variableValue := m.A
		m.directStore(variableAddress, variableValue)

		// BUMP LFPT,OF(LNM)  // increase a variable by a literal value
		literalValue := m.Registers.LNM
		variableAddress = m.Registers.LFPT
		variableValue = m.directLoad(variableAddress)
		m.directStore(variableAddress, literalValue+variableValue)
	case op.MDCONV: // convert MEVAL to decimal text
		return m.mdconv()
	case op.MDFIND: // hash an atom on to a chain
		return m.mdfind()
	case op.MDOP: // multiply or divide
		return m.mdop()
	case op.MDOUCH: // copy register C to the results stream
		return m.mdouch(m.host(stdout, stderr))
	case op.MDREAD: // read one character of the source text into C
		return m.mdread(m.host(stdout, stderr))
	default:
		return fmt.Errorf("assert(op != %q != %d): %w", w.Op, w.Op, ErrInvalidOp)
	}

	return nil
}
