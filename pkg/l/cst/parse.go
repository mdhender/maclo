// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package cst

import (
	"github.com/mdhender/maclo/pkg/l/stmt"
	"github.com/mdhender/maclo/pkg/l/token"
)

// Parse turns a token stream into one Line per source statement.
//
// It accumulates diagnostics instead of stopping at the first, and it recovers
// at statement granularity: a line it cannot make sense of gets its error and
// is skipped to the next newline, so one bad line costs one diagnostic rather
// than cascading through everything after it.
func Parse(toks []token.Token) (*File, token.Errors) {
	p := &parser{toks: toks}
	f := p.run()
	errs := p.errs
	for _, line := range f.Lines {
		errs.Merge(line.Errs)
	}
	return f, errs
}

type parser struct {
	toks []token.Token
	pos  int
	errs token.Errors
}

func (p *parser) run() *File {
	f := &File{}
	var pending []token.Token
	for !p.atEnd() {
		// A blank line is layout. It does not interrupt the trivia gathering,
		// which is what lets a heading sit a line above the statement it
		// introduces, the way the L source of ML/I writes them.
		if p.peek().Kind == token.Newline {
			p.next()
			continue
		}
		if trivia, ok := p.triviaLine(); ok {
			pending = append(pending, trivia...)
			continue
		}
		line := p.parseLine()
		line.Lead, pending = pending, nil
		f.Lines = append(f.Lines, line)
	}
	f.Tail = pending
	return f
}

// triviaLine consumes a line that holds nothing but headings and comments and
// returns them. A comment that shares a line with a statement is an argument,
// not trivia, and is left where it is.
func (p *parser) triviaLine() ([]token.Token, bool) {
	i := p.pos
	for i < len(p.toks) && p.toks[i].Kind.IsTrivia() {
		i++
	}
	if i == p.pos || i >= len(p.toks) || p.toks[i].Kind != token.Newline {
		return nil, false
	}
	trivia := p.toks[p.pos:i]
	p.pos = i + 1 // past the newline
	return trivia, true
}

func (p *parser) parseLine() *Line {
	line := &Line{Pos: p.peek().Pos, Head: stmt.Unknown}

	// Labels and statement prefixes, in either order and any number. The
	// manual documents neither arrangement; the corpus uses both.
	for {
		switch t := p.peek(); t.Kind {
		case token.Prefix:
			line.Prefixes = append(line.Prefixes, t)
			p.next()
			continue
		case token.LabelName:
			if line.Label != nil {
				line.Errs.Add(t.Pos, token.StageCST, "second label %q on one line: a statement takes one", t.Text)
			} else {
				label := t
				line.Label = &label
			}
			p.next()
			continue
		}
		break
	}

	if p.peek().Kind == token.Newline {
		line.EndPos = p.peek().Pos
		switch {
		case line.Label != nil:
			line.Errs.Add(line.Label.Pos, token.StageCST, "label %q has no statement on its line", line.Label.Text)
		case len(line.Prefixes) > 0:
			line.Errs.Add(line.Prefixes[0].Pos, token.StageCST, "statement prefix with no statement")
		}
		p.next()
		return line
	}

	if !p.parseHead(line) {
		line.EndPos = p.skipToNewline()
		return line
	}
	p.parseArgs(line, &line.Args, 0)
	line.EndPos = p.skipToNewline()
	return line
}

// parseHead resolves the statement, consuming one word or two. stmt.Lookup is
// called here and nowhere else, which is what keeps EXIT-the-keyword and
// EXIT FROM-the-statement from ever being confused.
func (p *parser) parseHead(line *Line) bool {
	t := p.peek()
	if t.Kind != token.Word {
		line.Errs.Add(t.Pos, token.StageCST, "expected a statement, found %s", describe(t))
		return false
	}
	words := []string{t.Text}
	if n := p.at(p.pos + 1); n.Kind == token.Word {
		words = append(words, n.Text)
	}
	kind, consumed, ok := stmt.Lookup(words...)
	if !ok {
		line.Errs.Add(t.Pos, token.StageCST, "%q is not a statement of L", t.Text)
		return false
	}
	line.Head, line.HeadPos = kind, t.Pos
	for range consumed {
		p.next()
	}
	return true
}

// parseArgs reads everything up to the newline into args. depth is the number
// of open parentheses, so a ')' can tell "the group ends here" from "there was
// no group".
func (p *parser) parseArgs(line *Line, args *[]*Arg, depth int) {
	for {
		t := p.peek()
		switch t.Kind {
		case token.Newline, token.EOF:
			if depth > 0 {
				line.Errs.Add(t.Pos, token.StageCST, "unclosed ( at the end of the line")
			}
			return

		case token.LParen:
			p.next()
			group := &Arg{Pos: t.Pos, Kind: ArgGroup, Raw: "("}
			p.parseArgs(line, &group.Children, depth+1)
			if p.peek().Kind == token.RParen {
				p.next()
			}
			*args = append(*args, group)

		case token.RParen:
			if depth > 0 {
				return
			}
			line.Errs.Add(t.Pos, token.StageCST, "unexpected ): no ( opened here")
			p.next()

		case token.Word:
			*args = append(*args, &Arg{Pos: t.Pos, Kind: ArgWord, Text: t.Text, Raw: t.Raw})
			p.next()
		case token.Number:
			*args = append(*args, &Arg{Pos: t.Pos, Kind: ArgNumber, Text: t.Text, Raw: t.Raw, Num: t.Num})
			p.next()
		case token.Quote:
			*args = append(*args, &Arg{Pos: t.Pos, Kind: ArgQuote, Text: t.Text, Raw: t.Raw})
			p.next()
		case token.BracketText:
			*args = append(*args, &Arg{Pos: t.Pos, Kind: ArgText, Text: t.Text, Raw: t.Raw})
			p.next()
		case token.Comment:
			*args = append(*args, &Arg{Pos: t.Pos, Kind: ArgComment, Text: t.Text, Raw: t.Raw})
			p.next()

		case token.Comma, token.Equals, token.Amp, token.Bar,
			token.Plus, token.Minus, token.Star, token.Slash:
			*args = append(*args, &Arg{Pos: t.Pos, Kind: ArgPunct, Punct: t.Kind, Raw: punctText(t.Kind)})
			p.next()

		case token.Prefix:
			// A prefix after the head belongs to a statement that follows
			// THEN. Recording it as an argument keeps this stage concrete;
			// ast/build.go is where it becomes part of the inner statement,
			// and where a prefix in any other position is an error.
			*args = append(*args, &Arg{Pos: t.Pos, Kind: ArgPrefix, Text: t.Text, Raw: t.Raw})
			p.next()
		case token.LabelName:
			line.Errs.Add(t.Pos, token.StageCST, "label %q in the middle of a statement", t.Text)
			p.next()
		case token.Heading:
			line.Errs.Add(t.Pos, token.StageCST, "a heading comment occupies a line by itself (lmap.txt 2.5)")
			p.next()

		default:
			line.Errs.Add(t.Pos, token.StageCST, "unexpected %s", describe(t))
			p.next()
		}
	}
}

// skipToNewline consumes the rest of the line and reports where it ended.
func (p *parser) skipToNewline() token.Position {
	for {
		t := p.peek()
		switch t.Kind {
		case token.EOF:
			return t.Pos
		case token.Newline:
			p.next()
			return t.Pos
		}
		p.next()
	}
}

func (p *parser) atEnd() bool { return p.peek().Kind == token.EOF }

func (p *parser) peek() token.Token { return p.at(p.pos) }

func (p *parser) at(i int) token.Token {
	if i >= len(p.toks) {
		return token.Token{Kind: token.EOF}
	}
	return p.toks[i]
}

func (p *parser) next() {
	if p.pos < len(p.toks) {
		p.pos++
	}
}

// describe names a token in a diagnostic the way a reader would say it.
func describe(t token.Token) string {
	switch t.Kind {
	case token.Word:
		return "the word " + t.Text
	case token.Number:
		return "the number " + t.Raw
	case token.LabelName:
		return "the label [" + t.Text + "]"
	case token.Quote:
		return "the character '" + t.Raw + "'"
	case token.BracketText:
		return "the text [" + t.Raw + "]"
	case token.Comment:
		return "a comment"
	case token.Heading:
		return "a heading"
	case token.Prefix:
		return "a statement prefix"
	case token.Newline:
		return "the end of the line"
	case token.EOF:
		return "the end of the file"
	}
	if s := punctText(t.Kind); s != "" {
		return "'" + s + "'"
	}
	return t.Kind.String()
}

func punctText(k token.Kind) string {
	switch k {
	case token.Comma:
		return ","
	case token.Equals:
		return "="
	case token.Amp:
		return "&"
	case token.Bar:
		return "|"
	case token.Plus:
		return "+"
	case token.Minus:
		return "-"
	case token.Star:
		return "*"
	case token.Slash:
		return "/"
	case token.LParen:
		return "("
	case token.RParen:
		return ")"
	}
	return ""
}
