// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

// Package scanner implements a simple scanner
package scanner

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
)

// Scanner is a scanner
type Scanner struct {
	input buffer
	pb    Char
}

func NewScanner(name string) (*Scanner, error) {
	input, err := os.ReadFile(name)
	if err != nil {
		return &Scanner{}, err
	}
	return NewScannerFromBytes(input), nil
}

// NewScannerFromBytes scans source that is already in memory, which is what a
// library caller has: it must not read or write files of its own.
func NewScannerFromBytes(input []byte) *Scanner {
	return &Scanner{input: newBuffer(input)}
}

func (s *Scanner) TestBuffer() error {
	bb := &bytes.Buffer{}
	b := s.input
	for {
		var ch Char
		ch, b = b.getChar()
		if ch.IsEOF() {
			_, _ = fmt.Fprintf(bb, "\n\nend-of-input\n\n")
			break
		}
		bb.WriteByte(ch.Char)
	}
	if err := os.WriteFile("scanner_buffer.txt", bb.Bytes(), 0644); err != nil {
		return err
	}
	return nil
}

func (s *Scanner) TestScanner() error {
	bb := &bytes.Buffer{}
	for _, tok := range s.Tokens() {
		_, _ = fmt.Fprintf(bb, "%4d:%3d %-12s %q\n", tok.Line, tok.Col, tok.Kind.String(), tok.String())
	}
	if err := os.WriteFile("scanner_tokens.txt", bb.Bytes(), 0644); err != nil {
		return err
	}
	return nil
}

func (s *Scanner) Tokens() []Token {
	var tokens []Token
	for !s.IsEOF() {
		tok := s.Next()
		tokens = append(tokens, tok)
		if tok.Kind == Error {
			break
		}
	}
	return tokens
}

func (s *Scanner) IsEOF() bool {
	return s.input.iseof()
}

// Next returns the next token from the input
func (s *Scanner) Next() Token {
	// skip leading whitespace
	ch := s.nextChar()
	for ch.IsSpace() {
		ch = s.nextChar()
	}

	// bail on end of input
	if ch.IsEOF() {
		return Token{Line: s.input.line, Col: s.input.col, Kind: EndOfInput}
	}

	if ch.IsComma() {
		return Token{Line: ch.Line, Col: ch.Col, Kind: Comma}
	}
	if ch.IsEOL() {
		return Token{Line: ch.Line, Col: ch.Col, Kind: EndOfLine}
	}
	if ch.IsQuote() {
		t := Token{Line: ch.Line, Col: ch.Col, Kind: QuotedText}
		var text []byte
		for {
			ch = s.nextChar()
			if ch.IsQuote() {
				break
			}
			if ch.IsEOL() {
				// unterminated quoted text
				t.Kind, t.Value.Error = Error, fmt.Errorf("%d:%d: unterminated quoted text", t.Line, t.Col)
				return t
			}
			text = append(text, ch.Char)
		}
		t.Value.QuotedText = string(text)
		return t
	}
	if ch.IsAlpha() {
		// macro, opcode, or variable
		t := Token{Line: ch.Line, Col: ch.Col}
		text := []byte{ch.Char}
		for s.input.isalnum() {
			ch = s.nextChar()
			text = append(text, ch.Char)
		}
		txt := string(text)
		if isMacro(txt) {
			t.Kind = Macro
			t.Value.Macro = txt
		} else if opc, ok := isOpCode(txt); ok {
			t.Kind = OpCode
			t.Value.OpCode = opc
		} else {
			t.Kind = Variable
			t.Value.Variable = txt
		}
		return t
	}
	if ch.IsDigit() || (ch.Char == '-' && s.input.isnum()) {
		// number or negative number
		t := Token{Line: ch.Line, Col: ch.Col, Kind: Number}
		text := []byte{ch.Char}
		for s.input.isnum() {
			ch = s.nextChar()
			text = append(text, ch.Char)
		}
		if n, err := strconv.Atoi(string(text)); err != nil {
			t.Kind, t.Value.Error = Error, fmt.Errorf("%d:%d: %q", t.Line, t.Col, err)
			return t
		} else {
			t.Value.Number = n
		}
		return t
	}
	if ch.Char == '[' {
		t := Token{Line: ch.Line, Col: ch.Col, Kind: Label}
		if ch = s.nextChar(); !ch.IsAlpha() {
			// first character of label must be alpha
			t.Kind, t.Value.Error = Error, fmt.Errorf("%d:%d: invalid label", t.Line, t.Col)
			return t
		}

		label := []byte{ch.Char}
		for {
			if ch = s.nextChar(); ch.Char == ']' {
				break
			} else if !ch.IsAlNum() {
				// all characters of label must be alphanumeric
				t.Kind, t.Value.Error = Error, fmt.Errorf("%d:%d: invalid label", t.Line, t.Col)
				return t
			}
			label = append(label, ch.Char)
		}
		t.Value.Label = string(label)
		return t
	}
	if ch.Char == '(' {
		// expression
		t := Token{Line: ch.Line, Col: ch.Col, Kind: Expression}
		expr := []byte{ch.Char}
		for {
			ch = s.nextChar()
			if ch.IsEOL() || ch.IsEOF() {
				t.Kind, t.Value.Error = Error, fmt.Errorf("%d:%d: unterminated expression", t.Line, t.Col)
				return t
			}
			expr = append(expr, ch.Char)
			if ch.Char == ')' {
				break
			}
		}
		t.Value.Expression = string(expr)
		return t
	}

	t := Token{Line: ch.Line, Col: ch.Col, Kind: Error}
	t.Value.Text = string(ch.Char)
	t.Value.Error = fmt.Errorf("%d:%d: unexpected input %q", ch.Line, ch.Col, string(ch.Char))
	return t
}

func (s *Scanner) nextChar() Char {
	ch := s.pb
	if ch.Char != 0 {
		s.pb.Char = 0
		return ch
	}
	ch, s.input = s.input.getChar()
	return ch
}
