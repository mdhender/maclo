// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package lmap

import (
	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/lowl/op"
)

// The block moving statements.
//
// The manual writes each of them out as a loop over single characters and then
// says that an L-map need not follow the description, "provided that the
// overall effect is not altered. In fact the whole purpose of the block moving
// statements is to allow efficient code specially tailored to the object
// machine to be inserted." LOWL has the two instructions this needs, so none
// of the three becomes a loop.
//
// FMOVE and BMOVE take their operands from two variables rather than from the
// instruction -- the source in SRCPT, the destination in DSTPT and the length
// in the accumulator -- which is why the declarations gained two names.
//
// The two stacking forms are the manual's own rewriting rather than this
// L-map's invention: MSTACK on the backwards stack is defined as a call of
// DECLF and a move to LFPT, and on the forwards stack as a call of BUMPFF and
// a move to where the stack pointer was. Those are subroutines of the logic
// being mapped, so the arrangement is that the storage allocator stays the
// program's business and the copying is the machine's.

// The names the manual's rewritings use. They belong to the program rather
// than to this L-map, which is why they are not the LO names beside them.
const (
	stackedBlock  = "DBUGPT" // where the backwards stack copy of a block is
	addressLeftIn = "TEMPT"  // where BACKSPACE without GIVING leaves an address
	bumpForwards  = "BUMPFF" // grows the forwards stack, checking for overflow
	dropBackwards = "DECLF"  // grows the backwards stack, checking for overflow
	freeForwards  = "FFPT"   // the first free word of the forwards stack
	freeBackwards = "LFPT"   // the last free word, which the backwards stack grows down from
	overflowLabel = "ERLOVF" // where arithmetic overflow is reported
)

// moveFrom copies a block, forwards or backwards.
//
// The backwards form exists for the case where the two fields overlap in the
// direction that would make a forwards copy read what it has already written.
func (m *mapper) moveFrom(t *ast.MoveFrom) {
	line := t.Position.Line
	m.load(line, t.From)
	m.p.emit(line, op.STV, word("SRCPT"), word("X"))
	m.load(line, t.To)
	m.p.emit(line, op.STV, word("DSTPT"), word("X"))
	m.load(line, t.Leng)
	if t.Backwards {
		m.p.emit(line, op.BMOVE)
		return
	}
	m.p.emit(line, op.FMOVE)
}

// mstackFrom stacks a block.
//
// The length is worked out once and kept, because it is wanted twice -- by the
// allocator and by the move -- and the expression that gives it may read
// something the allocator changes.
func (m *mapper) mstackFrom(t *ast.MStackFrom) {
	line := t.Position.Line
	m.load(line, t.Leng)
	m.p.emit(line, op.STV, word(nameLeng), word("X"))

	if t.On == ast.BStack {
		// CALL DECLF(leng)NM  /  MOVE FROM from TO LFPT LENG leng
		m.p.emit(line, op.LAV, word(nameLeng), word("X"))
		m.p.emit(line, op.GOSUB, word(dropBackwards), num(0))
		m.load(line, t.From)
		m.p.emit(line, op.STV, word("SRCPT"), word("X"))
		m.p.emit(line, op.LAV, word(freeBackwards), word("X"))
		m.p.emit(line, op.STV, word("DSTPT"), word("X"))
		m.p.emit(line, op.LAV, word(nameLeng), word("X"))
		m.p.emit(line, op.FMOVE)
		return
	}

	// SET LV4 = FFPT  /  CALL BUMPFF(leng)NM  /  MOVE FROM from TO LV4 LENG leng
	//
	// The manual's LV4 is the destination and nothing else, so it is written
	// straight into DSTPT and the allocator is free to move FFPT past it.
	m.p.emit(line, op.LAV, word(freeForwards), word("X"))
	m.p.emit(line, op.STV, word("DSTPT"), word("X"))
	m.p.emit(line, op.LAV, word(nameLeng), word("X"))
	m.p.emit(line, op.GOSUB, word(bumpForwards), num(0))
	m.load(line, t.From)
	m.p.emit(line, op.STV, word("SRCPT"), word("X"))
	m.p.emit(line, op.LAV, word(nameLeng), word("X"))
	m.p.emit(line, op.FMOVE)
}

// munstackFrom unstacks a block from the backwards stack.
//
// The manual is explicit that the move comes before the stack pointer is
// dropped, because the block being unstacked may be the top of the stack and
// dropping first would let the copy read storage that is no longer reserved.
func (m *mapper) munstackFrom(t *ast.MUnstackFrom) {
	line := t.Position.Line
	m.load(line, t.From)
	m.p.emit(line, op.STV, word("SRCPT"), word("X"))
	m.load(line, t.To)
	m.p.emit(line, op.STV, word("DSTPT"), word("X"))
	m.load(line, t.Leng)
	m.p.emit(line, op.STV, word(nameLeng), word("X"))
	m.p.emit(line, op.LAV, word(nameLeng), word("X"))
	m.p.emit(line, op.FMOVE)

	// SET LFPT = from + leng. SRCPT still holds the source: FMOVE reads the
	// two pointers and does not advance them.
	m.p.emit(line, op.LAV, word("SRCPT"), word("X"))
	m.p.emit(line, op.AAV, word(nameLeng))
	m.p.emit(line, op.STV, word(freeBackwards), word("X"))
}
