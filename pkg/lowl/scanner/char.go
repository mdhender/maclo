// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package scanner

import "bytes"

// Char is a character in the scanner
type Char struct {
	Line, Col int
	Char      byte
}

func (ch Char) IsAlNum() bool {
	return ch.IsAlpha() || ch.IsDigit()
}

func (ch Char) IsAlpha() bool {
	return 'A' <= ch.Char && ch.Char <= 'Z'
}

func (ch Char) IsComma() bool {
	return ch.Char == ','
}

func (ch Char) IsDelim() bool {
	return bytes.IndexByte([]byte{0, '\n', ' ', '\t', '.', ',', ';', ':', '(', ')', '+', '-', '*'}, ch.Char) != -1
}

func (ch Char) IsDigit() bool {
	return '0' <= ch.Char && ch.Char <= '9'
}

func (ch Char) IsEOF() bool {
	return ch.Char == 0
}

func (ch Char) IsEOL() bool {
	return ch.Char == '\n'
}

func (ch Char) IsQuote() bool {
	return ch.Char == '\''
}

func (ch Char) IsSpace() bool {
	return ch.Char == ' ' || ch.Char == '\t'
}
