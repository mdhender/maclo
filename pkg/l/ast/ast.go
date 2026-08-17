// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

// Package ast is the abstract syntax of L: one Go type per statement, with
// compound statements holding the statements they guard.
//
// This is where L stops resembling LOWL. pkg/lowl/ast is
// Node{Op, Parameters} - one flat node per source line, because LOWL has one
// opcode per line and no nesting. L has five paired constructs, real
// expressions, and statements whose shapes have nothing in common, so a flat
// node would record a parse without representing it.
//
// The package imports nothing but token. In particular it holds no pointers
// into the symbol table: sema keys its own maps by Node, the way go/types
// keeps Info.Defs and Info.Uses outside go/ast. That keeps a tree printable,
// comparable and testable without a resolver having run.
package ast

import (
	"github.com/mdhender/maclo/pkg/l/stmt"
	"github.com/mdhender/maclo/pkg/l/token"
)

// Node is anything with a place in the source.
type Node interface {
	Pos() token.Position
}

// Stmt is a statement of L.
type Stmt interface {
	Node
	// Kind reports which statement of L this is.
	Kind() stmt.Kind
	// Common returns the label, prefixes and comments every statement can
	// carry, so a walker does not need a type switch to reach them.
	Common() *Base
	stmtNode()
}

// Expr is a value: an arithmetic expression, a constant, or one of the macros
// that stands for one.
type Expr interface {
	Node
	exprNode()
}

// Base is embedded by every statement.
type Base struct {
	Position token.Position

	// Label is the [IDENT] written on this statement, or nil.
	Label *Ident

	// Prefixes are the /- ... -/ comments that mark this statement for
	// special treatment in an L-map: OVP where arithmetic can overflow, IN on
	// the initialisation assignments, CSS on a label reached from inside a
	// subroutine (lmap.txt appendix A). The list is open - the manual says so -
	// so nothing here interprets them.
	Prefixes []Prefix

	// Lead is the headings and standalone comments written above the
	// statement.
	Lead []Trivia
}

// Pos implements Node.
func (b *Base) Pos() token.Position { return b.Position }

// Common implements Stmt.
func (b *Base) Common() *Base { return b }

func (b *Base) stmtNode() {}

// HasPrefix reports whether text is one of the statement's prefixes.
func (b *Base) HasPrefix(text string) bool {
	for _, p := range b.Prefixes {
		if p.Text == text {
			return true
		}
	}
	return false
}

// Prefix is one /- ... -/ statement prefix.
type Prefix struct {
	Position token.Position
	Text     string
}

// Pos implements Node.
func (p Prefix) Pos() token.Position { return p.Position }

// TriviaKind separates the two comment forms that carry no argument.
type TriviaKind uint8

// enums for TriviaKind
const (
	// Heading is /+ ... +/, which marks the start of a logically distinct
	// piece of logic and occupies a line by itself.
	Heading TriviaKind = iota
	// Note is // ... // written on a line of its own.
	Note
)

// String implements the Stringer interface.
func (k TriviaKind) String() string {
	if k == Heading {
		return "heading"
	}
	return "note"
}

// Trivia is a comment written above a statement.
type Trivia struct {
	Position token.Position
	Kind     TriviaKind
	Text     string
}

// Pos implements Node.
func (t Trivia) Pos() token.Position { return t.Position }

// DataType is the type of a variable, an indirect address or a stacked value.
// The last two characters of a variable's name give its type: SW is a switch,
// PT a pointer, and anything else a number. CH is never a variable
// (lmap.txt 3.2), but it is a legal indirect address.
type DataType uint8

// enums for DataType
const (
	NoType DataType = iota
	CH
	NM
	PT
	SW
)

// String implements the Stringer interface.
func (d DataType) String() string {
	switch d {
	case CH:
		return "CH"
	case NM:
		return "NM"
	case PT:
		return "PT"
	case SW:
		return "SW"
	}
	return ""
}

// DataTypeOf maps a type suffix word to its DataType, reporting whether the
// word was one.
func DataTypeOf(word string) (DataType, bool) {
	switch word {
	case "CH":
		return CH, true
	case "NM":
		return NM, true
	case "PT":
		return PT, true
	case "SW":
		return SW, true
	}
	return NoType, false
}

// TypeOfName infers a variable's type from its name (lmap.txt 3.2). It is
// recorded rather than enforced: this front end resolves names and does not
// type check.
func TypeOfName(name string) DataType {
	if len(name) >= 2 {
		switch name[len(name)-2:] {
		case "SW":
			return SW
		case "PT":
			return PT
		}
	}
	return NM
}

// RelOp is a comparison. L has no less-than: the five are =, NE, GR, GE and LE
// (lmap.txt 4.2.1.1).
type RelOp uint8

// enums for RelOp
const (
	EQ RelOp = iota
	NE
	GR
	GE
	LE
)

// String implements the Stringer interface.
func (r RelOp) String() string {
	switch r {
	case EQ:
		return "="
	case NE:
		return "NE"
	case GR:
		return "GR"
	case GE:
		return "GE"
	case LE:
		return "LE"
	}
	return "?"
}

// RelOpOf maps a source token to a comparison. "=" arrives as punctuation and
// the other four as words, so both spellings are accepted here.
func RelOpOf(s string) (RelOp, bool) {
	switch s {
	case "=":
		return EQ, true
	case "NE":
		return NE, true
	case "GR":
		return GR, true
	case "GE":
		return GE, true
	case "LE":
		return LE, true
	}
	return EQ, false
}

// BinOp is an operator. Add and Sub are the arithmetic of L proper; Mul and
// Div are legal only inside OF, and And and Or only in SETSW and in the join
// of an IF condition. sema/check.go enforces both restrictions, which is what
// lets one Expr tree cover all of them.
type BinOp uint8

// enums for BinOp
const (
	Add BinOp = iota
	Sub
	Mul
	Div
	And
	Or
)

// String implements the Stringer interface.
func (o BinOp) String() string {
	switch o {
	case Add:
		return "+"
	case Sub:
		return "-"
	case Mul:
		return "*"
	case Div:
		return "/"
	case And:
		return "&"
	case Or:
		return "|"
	}
	return "?"
}

// StackName is one of the two stacks. The forwards stack grows from the start
// of workspace and the backwards stack from the end (lmap.txt 2.8).
type StackName uint8

// enums for StackName
const (
	FStack StackName = iota
	BStack
)

// String implements the Stringer interface.
func (s StackName) String() string {
	if s == BStack {
		return "BSTACK"
	}
	return "FSTACK"
}

// Program is a whole L source.
type Program struct {
	// Stmts is the top level: PRGSTART, the SECTIONs, and PRGEND. The
	// statements of a SECTION hang off it rather than appearing here.
	Stmts []Stmt
	// Tail is trivia written after the last statement. There is no matching
	// Lead: trivia before the first statement is that statement's own, the
	// same as anywhere else in the file.
	Tail []Trivia
}

// Pos implements Node.
func (p *Program) Pos() token.Position {
	if len(p.Stmts) > 0 {
		return p.Stmts[0].Pos()
	}
	return token.Position{}
}
