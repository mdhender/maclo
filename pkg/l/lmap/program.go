// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package lmap

import (
	"strconv"

	"github.com/mdhender/maclo/pkg/lowl/op"
)

// Program is a LOWL program in statement order, before it is rendered.
//
// It is a flat list because LOWL is flat: L's tree has already been walked by
// the time anything gets here, and a compound statement has become a run of
// branches and labels like every other. The list is what layout walks to work
// out where each word lands, which is what RL needs.
type Program struct {
	Stmts []Stmt

	// fixups are the RL statements whose distance is not known until the whole
	// program has been laid out. Each names the statement holding the RL and
	// the label it should measure to.
	fixups []fixup

	// gen numbers the labels the L-map invents. L's own labels are the
	// program's; these are the join points and loop heads that compound
	// statements need and L never named.
	gen int
}

// Stmt is one line of LOWL: an opcode and its operands, a label on a line of
// its own, or a blank line.
//
// A label is a statement rather than a field on the statement it precedes
// because that is what it is: pkg/lowl/cst makes a node of its own out of
// every "[NAME]" and the assembler emits no code for it, so a label and the
// word after it are at the same address either way.
type Stmt struct {
	Op    string // the mnemonic, or "" for a label or a blank line
	Label string // the [NAME] on this line; only meaningful when Op is ""
	Args  []Arg
	Line  int // the line of L this came from, or 0 for the L-map's own code
}

// ArgKind says how an operand is written, which is the only thing that
// separates the four: LOWL has no types at this level, only spellings.
type ArgKind uint8

// enums for ArgKind
const (
	ArgWord   ArgKind = iota // a name, or one of the single letter flags
	ArgNum                   // a decimal integer
	ArgQuoted                // 'text'
	ArgOF                    // OF(text), left for the assembler to evaluate
)

// Arg is one operand.
type Arg struct {
	Kind ArgKind
	Text string
	Num  int
}

func word(s string) Arg     { return Arg{Kind: ArgWord, Text: s} }
func num(n int) Arg         { return Arg{Kind: ArgNum, Num: n} }
func quoted(s string) Arg   { return Arg{Kind: ArgQuoted, Text: quotable(s)} }
func ofArg(expr string) Arg { return Arg{Kind: ArgOF, Text: expr} }

// String renders one operand the way LOWL spells it.
func (a Arg) String() string {
	switch a.Kind {
	case ArgNum:
		return strconv.Itoa(a.Num)
	case ArgQuoted:
		return "'" + a.Text + "'"
	case ArgOF:
		return "OF(" + a.Text + ")"
	}
	return a.Text
}

// quotable makes a string safe to sit between the quotes of a LOWL literal.
//
// There is no escape: pkg/lowl/scanner ends a quoted string at the first
// apostrophe and there is no way to write one inside. Nothing in the L source
// of ML/I contains one, so this is a guard rather than a translation, and it
// substitutes rather than dropping so that a message keeps its length.
func quotable(s string) string {
	out := []byte(s)
	for i := range out {
		if out[i] == '\'' {
			out[i] = '"'
		}
	}
	return string(out)
}

// emit appends one instruction.
func (p *Program) emit(line int, code op.Code, args ...Arg) {
	p.Stmts = append(p.Stmts, Stmt{Op: code.String(), Args: args, Line: line})
}

// label appends a label on a line of its own.
func (p *Program) label(name string) {
	p.Stmts = append(p.Stmts, Stmt{Label: name})
}

// blank appends an empty line. Generated LOWL is read by people, and the
// paragraphing of the L source is the only clue to its shape that survives.
func (p *Program) blank() {
	p.Stmts = append(p.Stmts, Stmt{})
}

// comment appends a comment. LOWL has no comment syntax, only the NB
// statement, whose argument has to be quoted.
func (p *Program) comment(line int, text string) {
	if text == "" {
		return
	}
	p.emit(line, op.NB, quoted(text))
}

// newLabel invents a label. The name is outside L's three-to-six character
// identifier convention on purpose: a generated name that could collide with
// one the program chose would be a bug that only showed up on some inputs.
func (p *Program) newLabel(prefix string) string {
	p.gen++
	return prefix + strconv.Itoa(p.gen)
}

// fixup is one RL whose distance is filled in after layout.
type fixup struct {
	stmt   int    // index into Stmts
	target string // the label the distance is measured to
	adjust int    // a constant added to the distance, for RL(name)+n
}

// rl appends an RL, with a placeholder distance for layout to fill in.
func (p *Program) rl(line int, target string, adjust int) {
	p.fixups = append(p.fixups, fixup{stmt: len(p.Stmts), target: target, adjust: adjust})
	p.emit(line, op.RL, word(target), ofArg("0"))
}
