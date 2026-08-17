// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

// Package token holds the vocabulary the whole L front end shares: source
// positions, diagnostics, and the tokens the scanner produces.
//
// It is the lowest package in pkg/l so that scanner, cst, ast and sema can all
// report through one Errors list without importing each other.
package token

import "fmt"

// Token is one lexical item.
//
// Like pkg/lowl/scanner.Token this is one struct with every payload rather
// than an interface, discriminated by Kind. Unlike that one it carries only
// two payload fields, because the scanner no longer decides what a word means.
type Token struct {
	Pos  Position
	Kind Kind

	// Text is the token's meaning: the identifier, the decoded quote or
	// bracket argument, the trimmed prefix or comment text. It is empty for
	// punctuation, whose Kind says everything.
	Text string

	// Raw is the token exactly as it appeared, brackets and all. It differs
	// from Text only for Quote and BracketText, where $ stands for a newline
	// (lmap.txt 3.3.2) and the listing wants to round-trip what was written
	// while a back end wants the character it means.
	Raw string

	// Num is the value of a Number token.
	Num int
}

// String implements the Stringer interface.
func (t Token) String() string {
	switch t.Kind {
	case Number:
		return fmt.Sprintf("%s %s %d", t.Pos, t.Kind, t.Num)
	case Word, LabelName:
		return fmt.Sprintf("%s %s %s", t.Pos, t.Kind, t.Text)
	case Quote, BracketText, Heading, Comment, Prefix:
		return fmt.Sprintf("%s %s %q", t.Pos, t.Kind, t.Raw)
	}
	return fmt.Sprintf("%s %s", t.Pos, t.Kind)
}

// IsWord reports whether the token is the word text.
func (t Token) IsWord(text string) bool { return t.Kind == Word && t.Text == text }
