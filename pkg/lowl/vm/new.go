// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm

import "github.com/mdhender/maclo/pkg/lowl/op"

// MaxNumberText is how many words New reserves for MDCONV to build the decimal
// text of a number in. Eleven characters hold the longest value ML/I allows in
// a macro expression, -2147483647, and one word holds one character.
const MaxNumberText = 12

// New - yes
func New() *VM {
	// when we start running the machine, the PC will be set to the first instruction in the program.
	m := &VM{PC: 0, Symbols: make(map[string]int)}
	m.Registers.LCH = 1
	m.Registers.LNM = 1

	// first instruction should be a halt.
	m.Core[m.PC], m.PC = Word{Op: op.HALT}, m.PC+1

	// initialized the reserved addresses
	m.Registers.DSTPT, m.PC = m.PC, m.PC+1
	m.Registers.FFPT, m.PC = m.PC, m.PC+1
	m.Registers.LFPT, m.PC = m.PC, m.PC+1
	m.Registers.PARNM, m.PC = m.PC, m.PC+1
	m.Registers.SRCPT, m.PC = m.PC, m.PC+1

	// MDCONV hands its answer back as a pointer into core, so the digits need
	// an address that neither the program nor the workspace will reuse. Here,
	// below the program, is the one place that is true of.
	m.Registers.NUMPT, m.PC = m.PC, m.PC+MaxNumberText

	return m
}

func (m *VM) SetWord(pc int, w Word) {
	m.Core[pc] = w
}
