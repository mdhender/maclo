// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package scanner

import "bytes"

// buffer is a buffer
type buffer struct {
	line, col int
	input     []byte
}

// newBuffer returns a buffer initialized from the input
func newBuffer(input []byte) buffer {
	if len(input) == 0 {
		return buffer{}
	}
	return buffer{
		line:  1,
		input: append([]byte{}, input...),
	}
}

func (b buffer) getChar() (Char, buffer) {
	if b.iseof() {
		return Char{Line: b.line, Col: b.col}, buffer{line: b.line, col: b.col}
	}

	ch := b.input[0]
	b.col, b.input = b.col+1, b.input[1:]

	if ch == '\r' && len(b.input) != 0 && b.input[0] == '\n' {
		ch, b.input = '\n', b.input[1:]
	}

	if ch == '\n' {
		return Char{Line: b.line, Col: b.col, Char: '\n'}, buffer{line: b.line + 1, col: 0, input: b.input}
	}

	return Char{Line: b.line, Col: b.col, Char: ch}, buffer{line: b.line, col: b.col, input: b.input}
}

func (b buffer) isalpha() bool {
	return len(b.input) != 0 && ('A' <= b.input[0] && b.input[0] <= 'Z')
}

func (b buffer) isalnum() bool {
	return b.isalpha() || b.isnum()
}

func (b buffer) isnum() bool {
	return len(b.input) != 0 && ('0' <= b.input[0] && b.input[0] <= '9')
}

func (b buffer) iseof() bool {
	return len(b.input) == 0
}

func (b buffer) isopenparen() bool {
	return !b.iseof() && (b.input[0] == '(')
}

func (b buffer) isspace() bool {
	return !b.iseof() && (b.input[0] == ' ' || b.input[0] == '\t')
}

func (b buffer) runOf(chars []byte) buffer {
	for !b.iseof() && bytes.IndexByte(chars, b.input[0]) != -1 {
		_, b = b.getChar()
	}
	return b
}
