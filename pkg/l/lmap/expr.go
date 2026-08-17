// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package lmap

import (
	"strconv"
	"strings"

	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/sema"
	"github.com/mdhender/maclo/pkg/l/token"
	"github.com/mdhender/maclo/pkg/lowl/op"
)

// Expressions.
//
// This is the part of the mapping the manual makes easy. An L expression is
// strictly left to right, has no precedence, no parentheses and no operators
// but plus and minus -- multiplication is a statement of its own, SCALE, and
// division is in the MD-logic. So the code for an expression is one load
// followed by one add or subtract per remaining term, and there is nothing to
// decide.
//
// The one place a temporary is needed is a term that is not a leaf: LOWL can
// add a variable or a literal to the accumulator and nothing else, so
// "something + IND(p)" has to put the something down while the indirect load
// happens. Nothing in ML/I is written that way -- an indirect load is always
// the first term -- so the code below is a guard rather than a path.

// operand is a term LOWL can reach without touching the accumulator: a
// variable it can name, or a literal it can spell.
type operand struct {
	name string // the variable, when this is a variable
	lit  Arg    // the literal, when it is not
}

func (o operand) isVar() bool { return o.name != "" }

// operandOf returns the term as something LAV or LAL can take, if it is one.
func (m *mapper) operandOf(e ast.Expr) (operand, bool) {
	switch t := e.(type) {
	case *ast.Ident:
		return m.identOperand(t)
	case *ast.IntLit:
		return operand{lit: num(t.Value)}, true
	case *ast.OF:
		return operand{lit: ofArg(m.ofText(t.Arg))}, true
	}
	return operand{}, false
}

// identOperand says what a name means where a value is wanted.
func (m *mapper) identOperand(id *ast.Ident) (operand, bool) {
	name := id.Text
	if markers[name] {
		// L calls these constants; this L-map cannot, because their values are
		// not known until the tables have been laid out. The prelude works
		// them out once and they are variables from then on.
		return operand{name: name}, true
	}
	if v, ok := constants[name]; ok {
		return operand{lit: num(v)}, true
	}
	if n, ok := m.sizeConstant(name); ok {
		return operand{lit: ofArg(strconv.Itoa(n) + "*LNM")}, true
	}
	if sym, ok := m.tab.Lookup(name); ok && sym.Kind == sema.Variable {
		return operand{name: name}, true
	}
	m.errs.Add(id.Position, token.StageLMap, "%s cannot be used as a value here", name)
	return operand{lit: num(0)}, true
}

// sizeConstant answers the constant every BLOCKDEC declares: the block's name
// with SZ on the end, standing for how much storage it occupies.
func (m *mapper) sizeConstant(name string) (int, bool) {
	if !strings.HasSuffix(name, "SZ") {
		return 0, false
	}
	n, ok := m.blockSize[strings.TrimSuffix(name, "SZ")]
	return n, ok
}

// load leaves the value of e in the accumulator.
func (m *mapper) load(line int, e ast.Expr) {
	if e == nil {
		m.errs.Add(token.Position{Line: line}, token.StageLMap, "an expression is missing")
		return
	}
	if o, ok := m.operandOf(e); ok {
		if o.isVar() {
			m.p.emit(line, op.LAV, word(o.name), word("X"))
		} else {
			m.p.emit(line, op.LAL, o.lit)
		}
		return
	}
	switch t := e.(type) {
	case *ast.AD:
		m.loadAddress(line, t.Name)
	case *ast.BlockRef:
		m.loadBlock(line, t.Name)
	case *ast.Ind:
		m.p.emit(line, op.LAI, word(m.address(line, t.Addr)), word("X"))
	case *ast.CharLit:
		m.p.emit(line, op.LAL, m.charArg(t))
	case *ast.Unary:
		// the only unary operators are plus and minus, and the manual says a
		// leading minus means nothing has been accumulated yet
		if t.Op == ast.Sub {
			m.p.emit(line, op.LAL, num(0))
			m.combine(line, ast.Sub, t.X)
			return
		}
		m.load(line, t.X)
	case *ast.Binary:
		switch t.Op {
		case ast.Add, ast.Sub:
			m.load(line, t.X)
			m.combine(line, t.Op, t.Y)
		default:
			m.errs.Add(t.Position, token.StageLMap, "%s has no meaning outside the OF macro", t.Op)
		}
	default:
		m.errs.Add(e.Pos(), token.StageLMap, "this cannot be evaluated into a register")
	}
}

// combine adds or subtracts e from what is already in the accumulator.
func (m *mapper) combine(line int, bop ast.BinOp, e ast.Expr) {
	if o, ok := m.operandOf(e); ok {
		switch {
		case bop == ast.Add && o.isVar():
			m.p.emit(line, op.AAV, word(o.name))
		case bop == ast.Add:
			m.p.emit(line, op.AAL, o.lit)
		case o.isVar():
			m.p.emit(line, op.SAV, word(o.name))
		default:
			m.p.emit(line, op.SAL, o.lit)
		}
		return
	}

	// The right hand term needs the accumulator to itself, so the left has to
	// go somewhere. Subtraction is done the wrong way round and then negated
	// rather than with a second temporary: one scratch variable is easier to
	// reason about than two, and this is not a path ML/I takes.
	spill := m.spill(line)
	m.p.emit(line, op.STV, word(spill), word("X"))
	m.load(line, e)
	if bop == ast.Add {
		m.p.emit(line, op.AAV, word(spill))
		return
	}
	m.p.emit(line, op.SAV, word(spill))
	m.p.emit(line, op.STV, word(spill), word("X"))
	m.p.emit(line, op.LAL, num(0))
	m.p.emit(line, op.SAV, word(spill))
}

// spill hands out the scratch variable for the term being put down. There are
// two, and a program that needed a third would be nesting non-leaf terms three
// deep, which L cannot express in one statement.
func (m *mapper) spill(line int) string {
	m.spilled++
	switch m.spilled {
	case 1:
		return nameSpill
	case 2:
		return nameSpill2
	}
	m.errs.Add(token.Position{Line: line}, token.StageLMap, "this expression nests too deeply for the L-map")
	return nameSpill2
}

// address returns the name of a variable holding the address e evaluates to.
//
// LOWL's indirect load and store name a variable rather than taking an
// address from the accumulator, so an address that is not already in one has
// to be put in one.
func (m *mapper) address(line int, e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		if o, ok := m.operandOf(id); ok && o.isVar() {
			return o.name
		}
	}
	m.load(line, e)
	m.p.emit(line, op.STV, word(nameTemp), word("X"))
	return nameTemp
}

// storeAddress is address for the left hand side of an assignment. It uses a
// scratch variable of its own so that the address survives the evaluation of
// the value, which may want the other one.
func (m *mapper) storeAddress(line int, e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		if o, ok := m.operandOf(id); ok && o.isVar() {
			return o.name
		}
	}
	m.load(line, e)
	m.p.emit(line, op.STV, word(nameStore), word("X"))
	return nameStore
}

// loadAddress emits the AD macro: the address of a name rather than what is
// stored there.
//
// The flag says whether the name is a table item or a variable. Two names are
// neither: the error block and the S-variable vector are areas the MD-logic
// owns and hands out, so AD of one of them is a variable the prelude filled
// in rather than a name the assembler knows.
func (m *mapper) loadAddress(line int, id *ast.Ident) {
	if id == nil {
		m.errs.Add(token.Position{Line: line}, token.StageLMap, "AD has no argument")
		return
	}
	switch id.Text {
	case "ERBLOC":
		m.p.emit(line, op.LAV, word(nameErrBl), word("X"))
		return
	case "SVEC":
		m.p.emit(line, op.LAV, word(nameSVec), word("X"))
		return
	}
	flag := "C"
	if sym, ok := m.tab.Lookup(id.Text); ok && sym.Kind == sema.Variable {
		flag = "D"
	}
	m.p.emit(line, op.LAA, word(id.Text), word(flag))
}

// loadBlock emits the BLOCK macro, which is AD of the first variable of a
// block. The manual says most L-maps treat the two the same, and this one
// does: a block is laid out as the run of DCLs its BLOCKDEC became, so its
// address is the address of the first of them.
func (m *mapper) loadBlock(line int, id *ast.Ident) {
	if id == nil {
		m.errs.Add(token.Position{Line: line}, token.StageLMap, "BLOCK has no argument")
		return
	}
	first, ok := m.blockFirst[id.Text]
	if !ok {
		m.errs.Add(id.Position, token.StageLMap, "%s is not a block, so it has no address", id.Text)
		return
	}
	m.p.emit(line, op.LAA, word(first), word("D"))
}

// The character constants LOWL names rather than spells.
//
// A quoted character in LOWL is one byte between apostrophes, which cannot say
// "newline" and cannot say "the character that stands for a deleted one". The
// assembler seeds a name for each of those, and this is which is which.
var namedChars = map[string]string{
	"\n": "NLREP",
	" ":  "SPREP",
	"\t": "TABREP",
	"\"": "QUTREP",
}

// charArg renders a character literal as a LOWL operand.
func (m *mapper) charArg(c *ast.CharLit) Arg {
	if name, ok := namedChars[c.Text]; ok {
		return word(name)
	}
	if len(c.Text) != 1 {
		m.errs.Add(c.Position, token.StageLMap, "%q is not a single character", c.Raw)
		return num(0)
	}
	return quoted(c.Text)
}

// isNamedChar reports whether a character literal has to be compared with CCN,
// which takes a name, rather than CCL, which takes a quoted character.
func isNamedChar(c *ast.CharLit) bool {
	_, ok := namedChars[c.Text]
	return ok
}

// ofText renders the argument of an OF macro as LOWL writes it.
//
// Two things happen on the way. The length macros for a pointer and for a
// switch become the one for a number, which is the data type identity the
// manual recommends and the one this L-map has taken: a pointer, a number and
// a switch all occupy one word here, and pkg/lowl/assembler only seeds names
// for LCH, LNM, LICH and LHV, so LPT and LSW would not resolve. And nothing is
// spaced, because pkg/lowl/assembler's OF parser accumulates anything that is
// not an operator into the operand, spaces included.
func (m *mapper) ofText(e ast.Expr) string {
	var b strings.Builder
	m.ofInto(&b, e)
	return b.String()
}

func (m *mapper) ofInto(b *strings.Builder, e ast.Expr) {
	switch t := e.(type) {
	case *ast.Ident:
		switch t.Text {
		case "LPT", "LSW":
			b.WriteString("LNM")
		default:
			b.WriteString(t.Text)
		}
	case *ast.IntLit:
		b.WriteString(strconv.Itoa(t.Value))
	case *ast.Binary:
		// There are no parentheses: pkg/lowl/scanner ends the expression at
		// the first closing one, so a nested group cannot be written. The
		// evaluator honours precedence, so a flat rendering is right as long
		// as a sum never sits inside a product, which L's own left to right
		// grammar makes impossible to write.
		m.ofInto(b, t.X)
		b.WriteString(t.Op.String())
		m.ofInto(b, t.Y)
	case *ast.Unary:
		if t.Op == ast.Sub {
			b.WriteString("0-")
		}
		m.ofInto(b, t.X)
	case *ast.OF:
		m.ofInto(b, t.Arg)
	default:
		m.errs.Add(e.Pos(), token.StageLMap, "the OF macro cannot take this")
		b.WriteString("0")
	}
}
