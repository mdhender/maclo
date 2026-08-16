// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package scanner

import (
	"fmt"
	"github.com/maloquacious/ml_i/pkg/lowl/op"
)

// Token is a token from the input
type Token struct {
	Line, Col int
	Kind      Kind
	Value     struct {
		Error      error
		Expression string
		Label      string
		Macro      string
		Number     int
		OpCode     op.Code
		QuotedText string
		Text       string
		Variable   string
	}
}

func (t Token) String() string {
	switch t.Kind {
	case Comma:
		return ","
	case EndOfInput:
		return "eol"
	case EndOfLine:
		return "\n"
	case Error:
		return fmt.Sprintf("%v", t.Value.Error)
	case Expression:
		return t.Value.Expression
	case Label:
		return t.Value.Label
	case Macro:
		return t.Value.Macro
	case Number:
		return fmt.Sprintf("%d", t.Value.Number)
	case OpCode:
		return t.Value.OpCode.String()
	case QuotedText:
		return t.Value.QuotedText
	case Text:
		return t.Value.Text
	case Unknown:
		return fmt.Sprintf("?%q?", t.Value.Text)
	case Variable:
		return t.Value.Variable
	}
	panic(fmt.Sprintf("assert(kind != %d)", t.Kind))
}
