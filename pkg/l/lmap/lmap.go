// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package lmap

import (
	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/sema"
	"github.com/mdhender/maclo/pkg/l/token"
	"github.com/mdhender/maclo/pkg/lowl/op"
)

// Map turns a resolved L program into a LOWL one.
//
// It takes the table as well as the tree because the tree does not say what a
// name is. L writes a variable, a label, a subroutine and a constant all the
// same way, and the code for "add this to the accumulator" is different for
// each, so the answer has to come from the pass that worked it out.
//
// The caller is expected to have checked that the program resolves. Map will
// run over one that does not, because reporting several problems beats
// reporting the first, but a program with an undefined name maps into LOWL
// with an undefined name in it and the assembler is where that will show up.
func Map(prog *ast.Program, tab *sema.Table) (*Program, token.Errors) {
	m := &mapper{
		p:          &Program{},
		tab:        tab,
		slot:       make(map[string]int),
		blockFirst: make(map[string]string),
		blockSize:  make(map[string]int),
	}
	m.collect(prog)
	m.program(prog)
	m.p.layout(&m.errs)
	return m.p, m.errs
}

// mapper is the state one mapping needs.
type mapper struct {
	p    *Program
	errs token.Errors
	tab  *sema.Table

	// The VARS SECTION, read before anything is emitted. slot is the word
	// offset of a variable inside the outermost block it is in, which is what
	// BACKSPACE is defined in terms of; the other two answer BLOCK(name) and
	// the size constant every BLOCKDEC declares.
	slot       map[string]int
	blockFirst map[string]string
	blockSize  map[string]int

	// The subroutine being emitted. EXIT names it and the assembler checks
	// that the name matches, so there is no way to emit two bodies at once.
	sub   string
	exits int

	// deferred are the parameter names held back until PARNM is declared, so
	// that the aliases read after the thing they alias.
	deferred []string

	spilled        int  // scratch variables in use in the expression being emitted
	entryTaken     bool // the L-map has emitted its own BEGIN
	markerLabelled bool // the first WITHS marker carries the label the markers come from
}

// The label and variable names the L-map introduces.
//
// They are all six characters beginning LO, which is the shape L's own
// identifiers do not have: the manual's convention is three to six characters
// and the MI-logic of ML/I uses none of these. A generated name that could
// collide with one the program chose would be a bug that showed up on some
// programs and not others.
const (
	nameTemp   = "LOTEMP" // scratch for a load through a computed address
	nameStore  = "LOSTOR" // scratch for the address an assignment stores through
	nameSpill  = "LOSPIL" // scratch for the left of an expression whose right is not a leaf
	nameSpill2 = "LOSPI2" // the second of those
	nameLeng   = "LOLENG" // the length of a block move, evaluated once
	nameErrBl  = "LOERBL" // the address of the error block, which AD(ERBLOC) means
	nameSVec   = "LOSVEC" // the address of the S-variable vector, which AD(SVEC) means
	nameCWith  = "CWITHS" // the first WITHS marker, which the other markers come from

	nameStartChain = "LOSCHN" // the CHAIN FROM runtime, start
	nameEndChain   = "LOECHN" // the CHAIN FROM runtime, end
	namePrintText  = "LOERPR" // MDERPR
	nameNumber     = "LONUM"  // MDNUM
	nameRead       = "LOREAD" // the READ statement
	nameOutput     = "LOOUTP" // the OUTPUTID statement
	nameHalt       = "LOHALT" // MDHALT, and MDABRT with it
	nameGoByClass  = "LOGOBC" // MDGOBC
	nameLayChain   = "LOLAYX" // the word after the layout character chain
)

// markers are the four constants L treats as predefined and this L-map makes
// variables of, because their values are not known until the tables are laid
// out. The prelude derives them from the first WITHS marker.
var markers = map[string]bool{
	"WTHSMK": true,
	"WITHMK": true,
	"EXCLMK": true,
	"SPCSMK": true,
}

// constants are the values this L-map gives L's predefined constants.
//
// Four of them are choices rather than facts. OPMK and LOCMK say whether an
// operation macro is global or local and STRMK and the two insert markers are
// tags in the same field, so all that is required of them is that they differ
// and that nothing else in that field collides; these are the values the
// published LOWL uses. TEXMAX and HTMAX are how much of a long piece of text
// an error message prints before it gives up, and are the same.
var constants = map[string]int{
	"TRUE":   1,
	"FALSE":  0,
	"ZEROPT": 0,
	"NULLPT": 0,
	"ENDCHN": 0,
	"OPMK":   0,
	"LOCMK":  1,
	"UINSMK": 2,
	"PINSMK": 3,
	"STRMK":  4,
	"TEXMAX": 60,
	"HTMAX":  26,
}

// collect reads the declarations before any code is emitted.
//
// Two things need it. A block's size is a constant the program may use
// anywhere, including before the block is declared, and BACKSPACE needs the
// offset of a variable inside the block that holds it.
func (m *mapper) collect(prog *ast.Program) {
	var walk func(stmts []ast.Stmt)
	walk = func(stmts []ast.Stmt) {
		for _, s := range stmts {
			switch t := s.(type) {
			case *ast.Section:
				walk(t.Body)
			case *ast.BlockDec:
				m.collectBlock(t, 0)
			}
		}
	}
	walk(prog.Stmts)
}

// collectBlock records one block and the ones inside it, and returns its size
// in words.
//
// The offsets it records are relative to the outermost block rather than to
// the block the variable is written in, because that is what BACKSPACE means
// by "the number of units of storage occupied by the variables preceding it".
// EDB sits inside SDB, and a BACKSPACE of one of EDB's variables is measured
// from the start of SDB.
func (m *mapper) collectBlock(b *ast.BlockDec, base int) int {
	n := 0
	for _, s := range b.Body {
		switch t := s.(type) {
		case *ast.Dec:
			if t.Name != nil {
				if _, seen := m.slot[t.Name.Text]; !seen {
					m.slot[t.Name.Text] = base + n
				}
				if b.Name != nil {
					if _, seen := m.blockFirst[b.Name.Text]; !seen {
						m.blockFirst[b.Name.Text] = t.Name.Text
					}
				}
			}
			n++
		case *ast.BlockDec:
			inner := m.collectBlock(t, base+n)
			if b.Name != nil && t.Name != nil {
				if _, seen := m.blockFirst[b.Name.Text]; !seen {
					m.blockFirst[b.Name.Text] = m.blockFirst[t.Name.Text]
				}
			}
			n += inner
		}
	}
	if b.Name != nil {
		m.blockSize[b.Name.Text] = n
	}
	return n
}

// program emits the whole thing, in the order the L source is written in.
//
// The order is not rearranged because it is already right: L puts its
// declarations first, its data SECTIONs next and its code last, which is what
// LOWL wants too. Only three things are inserted -- the variables this L-map
// needs, the initialisation code, and the MD-logic -- and each goes where the
// thing it belongs beside is.
func (m *mapper) program(prog *ast.Program) {
	for _, s := range prog.Stmts {
		switch t := s.(type) {
		case *ast.PrgStart:
			m.lead(t.Common())
			m.p.emit(t.Position.Line, op.PRGST, quoted("L"))
			m.p.blank()
		case *ast.PrgEnd:
			m.prelude()
			m.lead(t.Common())
			m.p.emit(t.Position.Line, op.PRGEN)
		case *ast.Section:
			m.section(t)
		default:
			m.errs.Add(s.Pos(), token.StageLMap, "%s is not a statement of the outermost level", s.Kind())
		}
	}
}

// section emits one SECTION.
func (m *mapper) section(s *ast.Section) {
	m.lead(s.Common())
	m.p.comment(s.Position.Line, s.Comment)
	m.p.blank()

	switch {
	case isDeclarative(s.Body):
		m.stmts(s.Body)
		m.p.blank()
		m.ownVariables(s.Position.Line)
	default:
		m.stmts(s.Body)
	}
	if s.EndLabel != nil {
		m.p.label(s.EndLabel.Text)
	}
	m.p.blank()
}

// isDeclarative reports whether a SECTION holds declarations rather than code
// or data. The VARS SECTION is the only one that does, and it is recognised by
// what is in it rather than by its name so that a program that renames it
// still works.
func isDeclarative(body []ast.Stmt) bool {
	seen := false
	for _, s := range body {
		switch s.(type) {
		case *ast.Dec, *ast.Equate, *ast.BlockDec:
			seen = true
		default:
			return false
		}
	}
	return seen
}

// stmts emits a run of statements.
func (m *mapper) stmts(body []ast.Stmt) {
	for _, s := range body {
		m.stmt(s)
	}
}

// stmt emits one statement: its leading comments, its label, its prefix, and
// then the code for the statement itself.
func (m *mapper) stmt(s ast.Stmt) {
	b := s.Common()
	m.lead(b)
	m.head(b)
	m.body(s)
}

// lead emits the comments written above a statement. LOWL has no comment
// syntax, so they become NB statements, which the assembler reads and emits
// nothing for.
func (m *mapper) lead(b *ast.Base) {
	if b == nil {
		return
	}
	for _, t := range b.Lead {
		if t.Text == "" {
			m.p.blank()
			continue
		}
		m.p.comment(t.Position.Line, t.Text)
	}
}

// head emits the label a statement carries and the code its prefixes call for.
//
// Only one prefix maps to anything. /-CSS-/ marks a label that is branched to
// from inside a subroutine without going through the exit mechanism, so the
// stack of return addresses has to be thrown away on arrival; that is what the
// CSS instruction is for. /-IN-/ and /-OVP-/ say where a statement is and what
// its arithmetic may do, and this L-map has no use for either.
func (m *mapper) head(b *ast.Base) {
	if b == nil {
		return
	}
	if b.Label != nil {
		m.checkName(b.Label)
		if m.takeEntry(b.Label.Text) {
			// the L-map has put its own initialisation here and taken the
			// label with it, so the statement itself is unlabelled
		} else {
			m.p.label(m.labelName(b.Label.Text))
		}
	}
	if b.HasPrefix("CSS") {
		m.p.emit(b.Position.Line, op.CSS)
	}
}

// takeEntry reports whether name is the entry label, and if it is, emits the
// initialisation code under it.
//
// The entry point is the one label the L-map cannot leave to the program. The
// MD-logic has to run before the MI-logic does, and the assembler starts the
// machine at BEGIN, so BEGIN is where the MD-logic has to be and the statement
// the program wrote it on follows rather than carries it. Nothing in ML/I
// branches to BEGIN, so nothing notices; a program that did would re-run its
// own initialisation, which is what it asked for.
func (m *mapper) takeEntry(name string) bool {
	if name != "BEGIN" || m.entryTaken {
		return false
	}
	m.entryTaken = true
	m.initialise()
	return true
}

// labelName is the LOWL spelling of an L label.
//
// They are the same, apart from the machine dependent ones: L branches to
// MDHALT, MDABRT and MDGOBC, which are labels of the MD-logic and so are this
// L-map's to name.
func (m *mapper) labelName(name string) string {
	switch name {
	case "MDHALT", "MDABRT":
		return nameHalt
	case "MDGOBC":
		return nameGoByClass
	}
	return name
}
