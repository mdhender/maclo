// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

// Package scanner turns L source into tokens.
//
// Scan is a pure function of bytes. It reads no file and writes to no stream,
// which is what keeps pkg/l out of the writeSites table in
// debug_artifacts_test.go: only cmd/macl and cmd/lcheck open anything, and
// only on a path the user named.
//
// The scanner does not classify keywords. pkg/lowl/scanner calls op.Lookup
// while it scans, so its lexer has to be edited whenever the language gains a
// mnemonic; here every identifier-shaped run comes out as token.Word and the
// parser decides what it is from where it sits. L makes that safe, because its
// keywords are two to eleven characters and its identifiers three to six
// (lmap.txt 2.3), so a short keyword can never be an identifier.
//
// Exactly one lexical decision survives, and it is unavoidable: see the
// comment on scanner.prev.
package scanner

import (
	"strconv"
	"strings"

	"github.com/mdhender/maclo/pkg/l/token"
)

// Scan turns src into tokens, accumulating diagnostics rather than stopping at
// the first one. The list always ends with a Newline and an EOF, so a parser
// never has to special-case a file whose last line has no terminator.
func Scan(src []byte) ([]token.Token, token.Errors) {
	s := &scanner{src: src, line: 1, col: 1}
	s.run()
	return s.toks, s.errs
}

type scanner struct {
	src  []byte
	pos  int
	line int
	col  int

	toks []token.Token
	errs token.Errors

	// prev is the last token emitted on the current line, and it exists for
	// one rule.
	//
	// A '[' is a label bracket in "[SKIP] SETSW SQSW = 0" and a string bracket
	// in "PRTEXT[SKIP]". The bytes are identical; only what precedes them
	// differs. So: if the token immediately before '[' is the word PRTEXT or
	// LID, the bracket opens text (lmap.txt 4.4.3, 6.1.2); otherwise it opens
	// a label (lmap.txt 2.7).
	//
	// One slot of state, reset at every newline, is the whole of the feedback
	// between this scanner and its parser. The alternative is a scanner with a
	// mode the parser sets, which is more machinery for the same two names.
	prev token.Token
}

func (s *scanner) run() {
	for {
		s.skipBlanks()
		if s.atEnd() {
			break
		}
		switch c := s.peek(); {
		case c == '\n':
			s.take()
			s.emit(token.Token{Pos: s.markBack(1), Kind: token.Newline})
			s.line, s.col = s.line+1, 1
		case c == '\'':
			s.scanQuote()
		case c == '/':
			s.scanSlash()
		case c == '[':
			s.scanBracket()
		case isDigit(c):
			s.scanNumber()
		case isUpper(c):
			s.scanWord()
		case isLower(c):
			s.scanLowerWord()
		default:
			s.scanPunct()
		}
	}
	// A file whose last line has no newline still ends a statement, so give
	// the parser the terminator it would otherwise have to invent.
	if n := len(s.toks); n == 0 || s.toks[n-1].Kind != token.Newline {
		s.emit(token.Token{Pos: s.mark(), Kind: token.Newline})
	}
	s.toks = append(s.toks, token.Token{Pos: s.mark(), Kind: token.EOF})
}

// --- the five interesting scans -------------------------------------------

// scanQuote reads the quote macro (lmap.txt 3.3.2).
//
// This case sits above the '/' case in run's switch, and that ordering is load
// bearing: the L source of ML/I contains a CHARMATCH whose arms include '*'
// and '/' on one line. Because a quote is recognised first, the '/' inside it
// never reaches the comment logic.
func (s *scanner) scanQuote() {
	pos := s.mark()
	s.take() // the opening '
	start := s.pos
	for !s.atEnd() && s.peek() != '\'' && s.peek() != '\n' {
		s.take()
	}
	if s.atEnd() || s.peek() == '\n' {
		s.errs.Add(pos, token.StageScanner, "unterminated quote: no closing ' before the end of the line")
		return
	}
	raw := string(s.src[start:s.pos])
	s.take() // the closing '
	s.emit(token.Token{Pos: pos, Kind: token.Quote, Raw: raw, Text: decodeDollar(raw)})
}

// scanSlash reads the three comment forms, or a division operator.
//
// Every terminator is two characters. Scanning to the next bare '/' would be
// wrong on the second line of the L source of ML/I, which is a comment whose
// text contains a '/'.
func (s *scanner) scanSlash() {
	pos := s.mark()
	if s.pos+1 >= len(s.src) {
		s.take()
		s.emit(token.Token{Pos: pos, Kind: token.Slash})
		return
	}
	var kind token.Kind
	var closer string
	switch s.src[s.pos+1] {
	case '+':
		kind, closer = token.Heading, "+/"
	case '-':
		kind, closer = token.Prefix, "-/"
	case '/':
		kind, closer = token.Comment, "//"
	default:
		// Division. It occurs nowhere in the corpus, but LICH is defined as
		// 1/LCH (lmap.txt 3.3.1), so it is part of the vocabulary. Whether a
		// slash is legal *here* is a question about OF, which the scanner
		// cannot answer; sema does it in check.go.
		s.take()
		s.emit(token.Token{Pos: pos, Kind: token.Slash})
		return
	}
	s.take()
	s.take()
	start := s.pos
	for {
		if s.atEnd() || s.peek() == '\n' {
			s.errs.Add(pos, token.StageScanner, "unterminated comment: no closing %s before the end of the line", closer)
			return
		}
		if s.pos+1 < len(s.src) && s.src[s.pos] == closer[0] && s.src[s.pos+1] == closer[1] {
			break
		}
		s.take()
	}
	raw := string(s.src[start:s.pos])
	s.take()
	s.take()
	s.emit(token.Token{Pos: pos, Kind: kind, Raw: raw, Text: strings.TrimSpace(raw)})
}

// scanBracket reads either a label or the argument of PRTEXT or LID. See the
// comment on scanner.prev for why one cannot be told from the other without
// looking back.
func (s *scanner) scanBracket() {
	if s.prev.Kind == token.Word && (s.prev.Text == "PRTEXT" || s.prev.Text == "LID") {
		s.scanBracketText()
		return
	}
	s.scanLabel()
}

// scanBracketText reads to the first ']'. Neither bracket occurs inside any
// bracket text in the corpus, so there is nothing to nest. Whitespace inside
// is significant: PRTEXT[)    ] is four spaces the listing has to keep.
func (s *scanner) scanBracketText() {
	pos := s.mark()
	s.take() // the [
	start := s.pos
	for !s.atEnd() && s.peek() != ']' && s.peek() != '\n' {
		s.take()
	}
	if s.atEnd() || s.peek() == '\n' {
		s.errs.Add(pos, token.StageScanner, "unterminated text: no closing ] before the end of the line")
		return
	}
	raw := string(s.src[start:s.pos])
	s.take() // the ]
	s.emit(token.Token{Pos: pos, Kind: token.BracketText, Raw: raw, Text: decodeDollar(raw)})
}

func (s *scanner) scanLabel() {
	pos := s.mark()
	s.take() // the [
	start := s.pos
	for !s.atEnd() && isAlnum(s.peek()) {
		s.take()
	}
	name := string(s.src[start:s.pos])
	if name == "" {
		s.errs.Add(pos, token.StageScanner, "empty label: [ must be followed by an identifier")
		return
	}
	if !isUpper(name[0]) {
		s.errs.Add(pos, token.StageScanner, "label %q does not start with a letter", name)
		return
	}
	if s.atEnd() || s.peek() != ']' {
		s.errs.Add(pos, token.StageScanner, "unterminated label: no closing ] after %q", name)
		return
	}
	s.take() // the ]
	s.emit(token.Token{Pos: pos, Kind: token.LabelName, Text: name, Raw: name})
}

func (s *scanner) scanNumber() {
	pos := s.mark()
	start := s.pos
	for !s.atEnd() && isDigit(s.peek()) {
		s.take()
	}
	text := string(s.src[start:s.pos])
	n, err := strconv.Atoi(text)
	if err != nil {
		s.errs.Add(pos, token.StageScanner, "number %q: %v", text, err)
		return
	}
	s.emit(token.Token{Pos: pos, Kind: token.Number, Num: n, Text: text, Raw: text})
}

// scanWord reads an identifier-shaped run and says nothing about what it means.
func (s *scanner) scanWord() {
	pos := s.mark()
	start := s.pos
	for !s.atEnd() && isAlnum(s.peek()) {
		s.take()
	}
	text := string(s.src[start:s.pos])
	s.emit(token.Token{Pos: pos, Kind: token.Word, Text: text, Raw: text})
}

// scanLowerWord consumes a whole word that begins with a lower case letter and
// reports it once. L is written in capitals (lmap.txt 2.1), and a file typed in
// the wrong case would otherwise produce one diagnostic per character.
func (s *scanner) scanLowerWord() {
	pos := s.mark()
	start := s.pos
	for !s.atEnd() && (isAlnum(s.peek()) || isLower(s.peek())) {
		s.take()
	}
	s.errs.Add(pos, token.StageScanner, "%q is not in capitals: L is written in upper case (lmap.txt 2.1)",
		string(s.src[start:s.pos]))
}

func (s *scanner) scanPunct() {
	pos, c := s.mark(), s.peek()
	var kind token.Kind
	switch c {
	case ',':
		kind = token.Comma
	case '(':
		kind = token.LParen
	case ')':
		kind = token.RParen
	case '=':
		kind = token.Equals
	case '&':
		kind = token.Amp
	case '|':
		kind = token.Bar
	case '+':
		kind = token.Plus
	case '-':
		kind = token.Minus
	case '*':
		kind = token.Star
	case ']':
		s.take()
		s.errs.Add(pos, token.StageScanner, "unexpected ]: no [ opened here")
		return
	case '$', ';':
		s.take()
		s.errs.Add(pos, token.StageScanner, "%q is only allowed inside a quote or PRTEXT text", string(c))
		return
	default:
		s.take()
		if isLower(c) {
			s.errs.Add(pos, token.StageScanner, "lower case %q: L is written in capitals (lmap.txt 2.1)", string(c))
		} else {
			s.errs.Add(pos, token.StageScanner, "unexpected character %q", string(c))
		}
		return
	}
	s.take()
	s.emit(token.Token{Pos: pos, Kind: kind})
}

// --- plumbing --------------------------------------------------------------

func (s *scanner) atEnd() bool { return s.pos >= len(s.src) }
func (s *scanner) peek() byte  { return s.src[s.pos] }
func (s *scanner) mark() token.Position {
	return token.Position{Line: s.line, Col: s.col}
}

// markBack is mark for a token whose bytes have already been consumed.
func (s *scanner) markBack(n int) token.Position {
	return token.Position{Line: s.line, Col: s.col - n}
}

func (s *scanner) take() {
	s.pos++
	s.col++
}

// skipBlanks consumes spaces and tabs, and folds a CR that precedes a LF.
// Layout is otherwise insignificant in L (lmap.txt 2.1); newline is not, so it
// is left for run to turn into a token.
func (s *scanner) skipBlanks() {
	for !s.atEnd() {
		switch s.peek() {
		case ' ', '\t':
			s.take()
		case '\r':
			if s.pos+1 < len(s.src) && s.src[s.pos+1] == '\n' {
				s.pos++ // fold it away without moving the column
				continue
			}
			s.errs.Add(s.mark(), token.StageScanner, "carriage return that does not precede a newline")
			s.take()
		default:
			return
		}
	}
}

// emit appends a token and remembers it. A Newline is remembered like anything
// else, which is what resets the bracket rule at the end of a line: a Newline
// is not a Word, so the next '[' opens a label.
func (s *scanner) emit(t token.Token) {
	s.toks = append(s.toks, t)
	s.prev = t
}

// decodeDollar turns the source form of a quote or bracket argument into the
// characters it stands for. Every character stands for itself except $, which
// is a newline (lmap.txt 3.3.2).
func decodeDollar(raw string) string {
	if !strings.ContainsRune(raw, '$') {
		return raw
	}
	return strings.ReplaceAll(raw, "$", "\n")
}

func isUpper(c byte) bool { return 'A' <= c && c <= 'Z' }
func isLower(c byte) bool { return 'a' <= c && c <= 'z' }
func isDigit(c byte) bool { return '0' <= c && c <= '9' }
func isAlnum(c byte) bool { return isUpper(c) || isDigit(c) }
