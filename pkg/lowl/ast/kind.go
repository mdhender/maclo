// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ast

import "fmt"

type Kind int

const (
	Expression Kind = iota
	Label
	Macro
	Number
	QuotedText
	Variable
)

// String implements the Stringer interface.
func (k Kind) String() string {
	switch k {
	case Expression:
		return "expression"
	case Label:
		return "label"
	case Macro:
		return "macro"
	case Number:
		return "number"
	case QuotedText:
		return "quotedText"
	case Variable:
		return "variable"
	}
	panic(fmt.Sprintf("assert(kind != %d)", k))
}
