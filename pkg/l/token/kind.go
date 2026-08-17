// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package token

import "fmt"

// Kind is the kind of token.
//
// There is no OpCode kind here, and that is the point. pkg/lowl/scanner
// classifies a word by calling op.Lookup while it scans, so the lexer has to
// be edited whenever the language gains a mnemonic. This scanner emits Word
// for every identifier-shaped run and lets the parser decide what it is from
// where it sits. L makes that safe: its keywords are two to eleven characters
// and its identifiers are three to six, so the short keywords can never be
// identifiers, and the rest are unambiguous by position.
type Kind uint8

// enums for Kind
const (
	Invalid Kind = iota

	// Word is an identifier-shaped run: a letter then letters or digits. Every
	// keyword, variable, label name, subroutine name and type suffix is one of
	// these until the parser says otherwise.
	Word
	// Number is a run of digits. A leading minus is punctuation, not part of
	// the number, because "A-1" and "A -1" have to scan the same way.
	Number
	// Quote is the quote macro, 'A' or 'MCDEF'. Text holds the argument with $
	// decoded to a newline; Raw holds it as written.
	Quote
	// BracketText is the argument of PRTEXT or LID, which is written in square
	// brackets and whose spaces are significant.
	BracketText
	// LabelName is the identifier inside a [LABEL] bracket.
	LabelName

	// Heading is a /+ ... +/ comment. It occupies a line by itself.
	Heading
	// Comment is a // ... // comment. It is trivia when it begins a line and a
	// real argument after SECTION, BLOCKDEC and a subroutine's EXIT.
	Comment
	// Prefix is a /- ... -/ statement prefix, such as OVP, IN or CSS. Text
	// holds the inner text with its surrounding spaces trimmed.
	Prefix

	Comma
	LParen
	RParen
	Equals
	Amp
	Bar
	Plus
	Minus
	Star
	Slash

	// Newline terminates a statement. Nothing in L continues across one.
	Newline
	// EOF is the last token of every scan.
	EOF
)

// String implements the Stringer interface.
func (k Kind) String() string {
	switch k {
	case Invalid:
		return "invalid"
	case Word:
		return "word"
	case Number:
		return "number"
	case Quote:
		return "quote"
	case BracketText:
		return "bracketText"
	case LabelName:
		return "labelName"
	case Heading:
		return "heading"
	case Comment:
		return "comment"
	case Prefix:
		return "prefix"
	case Comma:
		return "comma"
	case LParen:
		return "lparen"
	case RParen:
		return "rparen"
	case Equals:
		return "equals"
	case Amp:
		return "amp"
	case Bar:
		return "bar"
	case Plus:
		return "plus"
	case Minus:
		return "minus"
	case Star:
		return "star"
	case Slash:
		return "slash"
	case Newline:
		return "newline"
	case EOF:
		return "eof"
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// IsTrivia reports whether the token is a comment that carries no argument.
// A Prefix is not trivia: it belongs to the statement that follows it.
func (k Kind) IsTrivia() bool { return k == Heading || k == Comment }
