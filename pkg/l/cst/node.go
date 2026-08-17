// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

// Package cst is the concrete syntax of L: one node per source line, with the
// arguments of a line held as the tree they are written as.
//
// It is flat at statement level, like pkg/lowl/cst, because nothing in L spans
// a line - newline terminates a statement (lmap.txt 2.1) and there are no
// continuations. All the nesting in L comes from five paired constructs, and
// pairing them is a stack walk that ast/build.go does.
//
// Putting the pairing here would cost something specific: the cst would have
// to know that THEN at the end of a line opens a block while "THEN GO TO X"
// does not. That is a fact about the IF statement (lmap.txt 4.2.1), and it
// belongs in one place rather than two.
//
// Arguments do nest - IND(CHANPT+OF(2*LNM))NM - so those are a tree. It is a
// concrete one: it records "the word AD, a parenthesised group, the word PT"
// without deciding that the three of them are an address constant.
package cst

import (
	"github.com/mdhender/maclo/pkg/l/stmt"
	"github.com/mdhender/maclo/pkg/l/token"
)

// ArgKind is the kind of argument node.
type ArgKind uint8

// enums for ArgKind
const (
	ArgWord ArgKind = iota
	ArgNumber
	ArgQuote
	// ArgText is the bracketed argument of PRTEXT or LID.
	ArgText
	// ArgComment is a // ... // comment used as an argument, which happens
	// after SECTION, after BLOCKDEC, and after a subroutine's EXIT.
	ArgComment
	// ArgPrefix is a /- ... -/ statement prefix found after the statement
	// head, which happens when a one-line IF guards a prefixed statement:
	// "IF NEGVAL = 1 THEN /-OVP-/SET MEVAL = -MEVAL". The prefix belongs to
	// the statement after THEN, and ast/build.go attaches it there. The manual
	// documents neither the position nor the case.
	ArgPrefix
	// ArgPunct is one of , = & | + - * /. Punct says which.
	ArgPunct
	// ArgGroup is a parenthesised run. Children holds what was inside.
	ArgGroup
)

// String implements the Stringer interface.
func (k ArgKind) String() string {
	switch k {
	case ArgWord:
		return "word"
	case ArgNumber:
		return "number"
	case ArgQuote:
		return "quote"
	case ArgText:
		return "text"
	case ArgComment:
		return "comment"
	case ArgPrefix:
		return "prefix"
	case ArgPunct:
		return "punct"
	case ArgGroup:
		return "group"
	}
	return "arg"
}

// Arg is one argument node, as written.
//
// Note what is deliberately absent: there is no "call" kind folding
// NAME ( ... ) SUFFIX into one node. Layout is insignificant in L, so
// "STACK IDPT (PT)" and "STACK PARSW(SW)" must parse the same way, and both
// are a word followed by a group. Folding here would make the space
// meaningful; ast/build.go does the folding where it knows the context.
type Arg struct {
	Pos      token.Position
	Kind     ArgKind
	Text     string     // the identifier, or the decoded quote or bracket text
	Raw      string     // the token as written
	Num      int        // ArgNumber
	Punct    token.Kind // ArgPunct
	Children []*Arg     // ArgGroup
}

// Line is one source line: at most one statement, with whatever labelled it,
// prefixed it, or was written above it.
type Line struct {
	// Pos is the start of the line's first meaningful token.
	Pos token.Position

	// Lead holds the headings and standalone comments written above this line.
	// They belong to the statement that follows them (lmap.txt 2.5).
	Lead []token.Token

	// Label is the [IDENT] on this line, if any.
	Label *token.Token

	// Prefixes are the /- ... -/ statement prefixes, in source order.
	//
	// They may appear before or after the label, and in the L source of ML/I
	// both orders occur: the INVALS section writes "[BEGIN] /-IN-/SET" and
	// every CSS-marked label is written "/-CSS-/[FNCTEX] CALL". The manual
	// shows neither (it only ever writes "/- OVP -/ SET"), so the parser
	// accepts any number on either side.
	Prefixes []token.Token

	// Head is the statement, and HeadPos is where its first word sat. Head is
	// stmt.Unknown on a line the parser could not make sense of.
	Head    stmt.Kind
	HeadPos token.Position

	// Args is everything after the statement head.
	Args []*Arg

	// EndPos is the newline that terminated the line. It is what a diagnostic
	// about a missing argument points at, so "expected a value" lands where
	// the value should have been rather than at the start of the statement.
	EndPos token.Position

	// Errs are the diagnostics for this line. The parser records them here and
	// moves to the next line rather than stopping.
	Errs token.Errors
}

// HasPrefix reports whether text is one of the line's statement prefixes.
func (l *Line) HasPrefix(text string) bool {
	for _, p := range l.Prefixes {
		if p.Text == text {
			return true
		}
	}
	return false
}

// File is the whole of a parsed source.
type File struct {
	Lines []*Line
	// Tail holds trivia written after the last statement, so a listing can
	// round-trip a file that ends in a comment.
	Tail []token.Token
}
