// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package cst

import "github.com/maloquacious/ml_i/pkg/lowl/op"

type Node struct {
	Line, Col  int
	Kind       Kind
	Error      error
	OpCode     op.Code
	Number     int
	String     string
	Parameters []*Node
}

type Kind int

const (
	Error Kind = iota
	Expression
	Label
	Macro
	Number
	OpCode
	Parameter
	QuotedText
	String
	Text // todo: is this used?
	Variable
)
