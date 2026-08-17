// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package lmap

import (
	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/token"
	"github.com/mdhender/maclo/pkg/lowl/op"
)

// The VARS SECTION.
//
// DEC becomes DCL and EQUATE becomes EQU, and the only work is in what is not
// written down. A BLOCKDEC declares that its variables are stored contiguously
// and that a constant giving its size exists; LOWL has no such statement, so
// the block becomes the run of DCLs it contained, in order, and the size is
// substituted wherever the constant was used. The contiguity is then a
// property of the order the DCLs were emitted in, which is why nothing is
// allowed to sort them.
//
// The INIT clause is dropped. Every initial value in the L source of ML/I is
// assigned again by the INVALS SECTION, which is the dynamic initialisation
// the manual's chapter 5 puts beside the static kind, and a DCL is zero to
// begin with.

// parameterNames are the three names a subroutine's argument can arrive in.
//
// LOWL has one: the SUBR statement can store the accumulator into PARNM and
// nothing else. The manual says the three may be equated to one another on an
// L-map that does not distinguish data types, because ML/I never has more than
// one parameter in existence at a time, so that is what happens here.
var parameterNames = map[string]bool{"PARPT": true, "PARSW": true}

// dec emits one variable declaration.
func (m *mapper) dec(t *ast.Dec) {
	if t.Name == nil {
		return
	}
	if parameterNames[t.Name.Text] {
		// held back until PARNM itself has been declared, because an alias is
		// easier to read after the thing it aliases
		m.deferred = append(m.deferred, t.Name.Text)
		return
	}
	m.checkName(t.Name)
	m.p.emit(t.Position.Line, op.DCL, word(t.Name.Text))
}

// equate emits one EQUATE. LOWL writes the operands in the same order L does.
func (m *mapper) equate(t *ast.Equate) {
	if t.Name == nil || t.To == nil {
		return
	}
	m.checkName(t.Name)
	m.p.emit(t.Position.Line, op.EQU, word(t.Name.Text), word(t.To.Text))
}

// blockDec emits the variables of a block, and the ones of any block inside
// it, as one contiguous run.
func (m *mapper) blockDec(t *ast.BlockDec) {
	m.p.comment(t.Position.Line, t.Comment)
	m.p.comment(t.Position.Line, "the following are stored contiguously")
	m.stmts(t.Body)
	if t.EndLabel != nil {
		m.p.label(t.EndLabel.Text)
	}
	m.p.comment(t.EndPos.Line, "end of the contiguous block")
	m.p.blank()
}

// ownVariables declares what the L-map needs and L does not.
//
// Six are scratch: LOWL has no indirect store through the accumulator and no
// three-address arithmetic, so an address or a partial result sometimes has to
// be put down. Two hold addresses the MD-logic hands out rather than the
// assembler: the error block and the S-variable vector are areas rather than
// table items, so AD of either is a variable this L-map fills in. And four are
// the markers L calls constants -- their values fall out of where the tables
// landed, so they cannot be known until the program has been assembled, and
// the prelude works them out from the first WITHS marker.
func (m *mapper) ownVariables(line int) {
	for _, name := range m.deferred {
		m.p.emit(line, op.EQU, word(name), word("PARNM"))
	}
	m.deferred = nil

	m.p.blank()
	m.p.comment(line, "variables the L-map needs and L does not")
	m.p.emit(line, op.DCL, word("SRCPT"))
	m.p.emit(line, op.DCL, word("DSTPT"))
	m.p.emit(line, op.DCL, word(nameTemp))
	m.p.emit(line, op.DCL, word(nameStore))
	m.p.emit(line, op.DCL, word(nameSpill))
	m.p.emit(line, op.DCL, word(nameSpill2))
	m.p.emit(line, op.DCL, word(nameLeng))
	m.p.emit(line, op.DCL, word(nameNewLine))
	m.p.emit(line, op.DCL, word(nameErrBl))
	m.p.emit(line, op.DCL, word(nameSVec))
	for _, name := range markerOrder {
		m.p.emit(line, op.DCL, word(name))
	}
}

// markerOrder is the order the four markers are declared and derived in. It
// is fixed rather than taken from a map so that the generated LOWL does not
// change from one run to the next.
var markerOrder = []string{"WTHSMK", "WITHMK", "EXCLMK", "SPCSMK"}

// checkName reports a name that LOWL would read as an opcode.
//
// pkg/lowl/scanner classifies a bare word by looking it up in the mnemonic
// table, so a variable called GO or a label called CON would be scanned as an
// instruction and the line would fall apart in a way that is hard to read back
// to its cause. L's own identifiers are three to six characters, which
// overlaps the mnemonics, so this is worth saying plainly.
func (m *mapper) checkName(id *ast.Ident) {
	if id == nil {
		return
	}
	if _, isOpcode := op.Lookup(id.Text); isOpcode {
		m.errs.Add(id.Position, token.StageLMap, "%s is a LOWL mnemonic, so it cannot be a name in the generated program", id.Text)
	}
}
