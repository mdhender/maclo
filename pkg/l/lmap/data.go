// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package lmap

import (
	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/token"
	"github.com/mdhender/maclo/pkg/lowl/op"
)

// The data SECTIONs.
//
// These are the tables ML/I searches: the names of its own operation macros
// and the delimiters, keywords and layout characters those macros are written
// in terms of. Everything in them is one of four things -- a number, a run of
// characters, a link to somewhere else in the tables, or a marker -- and LOWL
// has an item for each.
//
// The link is the interesting one. RL holds the distance to what it points at
// rather than the address, because a table is walked by adding the word just
// read to the address it came from, and a distance survives being copied. So
// an RL is emitted with its distance left blank and layout fills it in, and
// the assembler then checks the answer against its own.
//
// Two statements have no data of their own and stand for tables the L-map has
// to supply: LAYCHAIN, which is the list of characters this implementation
// treats as layout, and HETABLES, which is the hash table and the areas beside
// it. Both are in chapter 6 of the manual as things the implementor writes.

// dc emits one table item, or several: the arguments of a DC are laid out in
// the order they are written and each of them is one or more words.
func (m *mapper) dc(t *ast.DC) {
	for _, arg := range t.Args {
		m.dataItem(t.Position.Line, arg)
	}
}

// dataItem emits one argument of a DC.
func (m *mapper) dataItem(line int, e ast.Expr) {
	switch t := e.(type) {
	case *ast.RL:
		adjust := 0
		if t.Adjust != nil {
			if o, ok := m.operandOf(t.Adjust); ok && !o.isVar() && o.lit.Kind == ArgNum {
				adjust = o.lit.Num
			} else {
				m.errs.Add(t.Position, token.StageLMap, "the adjustment on RL has to be a plain number")
			}
		}
		if t.Name == nil {
			return
		}
		m.p.rl(line, t.Name.Text, adjust)
	case *ast.LID:
		m.lid(line, t.Text)
	case *ast.CharLit:
		m.charItem(line, t)
	default:
		m.p.emit(line, op.CON, m.mustLiteral(e))
	}
}

// lid emits a literal identifier: the length of the text and then the text.
//
// That shape is why the tables can be searched with one subroutine. CMPARE is
// given a pointer to the word before the length and compares what follows with
// the atom it has just read, so every name in the tables -- a delimiter, a
// keyword, the name of an operation macro -- is laid out this way.
func (m *mapper) lid(line int, text *ast.TextLit) {
	if text == nil {
		return
	}
	m.p.emit(line, op.CON, ofArg(itoa(len(text.Text))+"*LCH"))
	if text.Text == "" {
		return
	}
	m.stringItem(line, text.Text)
}

// stringItem emits a run of characters.
//
// A quoted string in LOWL cannot hold a newline or a space that has to be
// distinguished from the layout of the source, so a text made of one such
// character becomes the named item instead. Longer ones are split at the same
// places, which keeps every character in exactly one word and so keeps the
// distances right.
func (m *mapper) stringItem(line int, text string) {
	var run []rune
	flush := func() {
		if len(run) != 0 {
			m.p.emit(line, op.STR, quoted(string(run)))
			run = run[:0]
		}
	}
	for _, r := range text {
		if name, ok := namedChars[string(r)]; ok {
			flush()
			m.p.emit(line, op.NCH, word(name))
			continue
		}
		run = append(run, r)
	}
	flush()
}

// charItem emits a single character written as an atom of a DC.
func (m *mapper) charItem(line int, c *ast.CharLit) {
	if name, ok := namedChars[c.Text]; ok {
		m.p.emit(line, op.NCH, word(name))
		return
	}
	m.stringItem(line, c.Text)
}

// opMac emits one entry of the table of operation macro names.
//
// The entry has to look like every other name in the tables, because the same
// search finds it: a hash chain link, an alternative-name link, and then the
// literal identifier. What follows the name is what makes it an operation
// macro -- the delimiter structure it is written in terms of, the fact that it
// is a macro rather than one of the other four constructions, whether it is
// global or local, and which of them it is.
func (m *mapper) opMac(t *ast.OpMac) {
	line := t.Position.Line
	if t.Name == nil {
		m.errs.Add(t.Position, token.StageLMap, "OPMAC wants a name")
		return
	}
	m.p.emit(line, op.HASH, quoted(t.Name.Text))
	m.p.emit(line, op.CON, num(0))
	m.lid(line, &ast.TextLit{Position: t.Name.Position, Text: t.Name.Text})
	if t.Name2 != nil {
		// A composite name: two atoms with nothing between them, which is what
		// the WITHS marker says. It is labelled because it is the only place
		// the marker's value can be read from, and the four markers the
		// prelude derives all come from it.
		if !m.markerLabelled {
			m.markerLabelled = true
			m.p.label(nameCWith)
		}
		m.p.emit(line, op.WTHS)
		m.lid(line, &ast.TextLit{Position: t.Name2.Position, Text: t.Name2.Text})
	}
	m.dataItem(line, t.Dels)
	m.p.emit(line, op.CON, num(constructionMacro))
	m.p.emit(line, op.CON, m.mustLiteral(t.Marker))
	m.p.emit(line, op.CON, m.mustLiteral(t.Number))
}

// constructionMacro is the value of the field that says which of ML/I's five
// constructions a name is. An operation macro is always a macro.
const constructionMacro = 1

// layChain emits the chain of layout characters.
//
// The manual leaves this to the implementor because which characters are
// layout is a property of the machine's character set rather than of ML/I.
// Four are wanted here: the space, the newline, the tab, and the marker that
// stands for a character deleted from the input. Each entry is a link to the
// next, a link to the character itself, and the keyword a user writes to name
// it, laid out like every other name in the tables.
//
// The chain runs on into whatever the L source wrote next, which is the entry
// for SPACES: the last link is a distance to the word after the ones emitted
// here, and the label on it exists only to measure to.
func (m *mapper) layChain(line int) {
	m.p.comment(line, "the layout characters of this implementation")
	for i, c := range layoutChars {
		next := nameLayChain
		if i+1 < len(layoutChars) {
			next = c.nextLabel
		}
		if c.label != "" {
			m.p.label(c.label)
		}
		m.p.rl(line, next, 0)
		m.p.rl(line, c.repLabel, 0)
		m.lid(line, &ast.TextLit{Position: token.Position{Line: line}, Text: c.keyword})
		m.p.emit(line, op.CON, num(0))
	}
	m.p.label(nameLayChain)
}

// layoutChar is one entry of the layout character chain: the keyword a user
// writes for it, the label on the word holding the character itself, and the
// label the entry before it measures its link to.
type layoutChar struct {
	label     string
	nextLabel string
	repLabel  string
	keyword   string
}

// The four layout characters, in the order the chain walks them. The labels
// are the L-map's own, and the representations they point at are emitted by
// HETABLES.
var layoutChars = []layoutChar{
	{label: "", nextLabel: "LOLYNL", repLabel: "LORSPA", keyword: "SPACE"},
	{label: "LOLYNL", nextLabel: "LOLYTB", repLabel: "LORNL", keyword: "NL"},
	{label: "LOLYTB", nextLabel: "LOLYSL", repLabel: "LORTAB", keyword: "TAB"},
	{label: "LOLYSL", nextLabel: "", repLabel: "LORSL", keyword: "SL"},
}

// heTables emits the tables the manual calls machine dependent: the
// representations of the layout characters, and the hash table.
//
// The hash table is a set of chain heads, one per chain, which the assembler
// lays out from the THASH statement because it and the machine have to agree
// on which chain a name lands on. Four words follow it, one per construction
// other than the stop marker, each holding the lowest address at which a
// definition of that construction is still valid; the code that fills them in
// stops at the fifth word, which is why that one is not zero.
func (m *mapper) heTables(line int) {
	m.p.comment(line, "the representations of the layout characters")
	for _, c := range layoutChars {
		m.p.label(c.repLabel)
		m.p.emit(line, op.NCH, word(layoutRep[c.repLabel]))
	}
	m.p.blank()
	m.p.comment(line, "the hash table, and the limits on local definitions")
	m.p.label("GHSHTB")
	m.p.emit(line, op.THASH)
	for i := 0; i < constructionsWithScope; i++ {
		m.p.emit(line, op.CON, num(0))
	}
	m.p.emit(line, op.CON, num(allWarningsOn))
}

// layoutRep says which character each entry of the layout chain stands for.
// The names are the assembler's: it seeds one for every character LOWL cannot
// write between apostrophes.
var layoutRep = map[string]string{
	"LORSPA": "SPREP",
	"LORNL":  "NLREP",
	"LORTAB": "TABREP",
	"LORSL":  "SLREP",
}

// constructionsWithScope is how many of ML/I's five constructions have a
// lowest valid address kept for them. The stop marker does not.
const constructionsWithScope = 4

// allWarningsOn is the initial value of the global warning switch, and the
// word the code that fills in the limits stops at. Every bit that selects a
// construction is set.
const allWarningsOn = 7
