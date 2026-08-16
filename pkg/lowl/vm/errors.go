// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm

import "fmt"

var (
	ErrCycles          = fmt.Errorf("too many cycles")
	ErrHalted          = fmt.Errorf("halted")
	ErrInvalidOp       = fmt.Errorf("invalid op")
	ErrNoHost          = fmt.Errorf("no host")
	ErrNoSymbol        = fmt.Errorf("undefined symbol")
	ErrNotExecutable   = fmt.Errorf("not executable")
	ErrNotImplemented  = fmt.Errorf("not implemented")
	ErrNumberTooLong   = fmt.Errorf("number too long")
	ErrQuit            = fmt.Errorf("quit")
	ErrStackOverflow   = fmt.Errorf("stack overflow")
	ErrStackUnderflow  = fmt.Errorf("stack underflow")
	ErrSystemVariables = fmt.Errorf("bad system variable block")
)
