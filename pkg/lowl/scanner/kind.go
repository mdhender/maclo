// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package scanner

import "fmt"

// Kind is the kind of token
type Kind int

// enums for Kind
const (
	Unknown Kind = iota
	Comma
	Error
	Expression
	Label
	Macro
	Number
	OpCode
	QuotedText
	Text
	Variable
	EndOfLine
	EndOfInput
)

// String implements the Stringer interface.
func (k Kind) String() string {
	switch k {
	case Comma:
		return "comma"
	case EndOfInput:
		return "endOfInput"
	case EndOfLine:
		return "endOfLine"
	case Error:
		return "error"
	case Expression:
		return "expression"
	case Label:
		return "label"
	case Macro:
		return "macro"
	case Number:
		return "number"
	case OpCode:
		return "opCode"
	case QuotedText:
		return "quotedText"
	case Text:
		return "text"
	case Unknown:
		return "unknown"
	case Variable:
		return "variable"
	}
	panic(fmt.Sprintf("assert(kind != %d)", k))
}
