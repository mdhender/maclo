// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ast

import (
	"github.com/mdhender/maclo/pkg/l/cst"
	"github.com/mdhender/maclo/pkg/l/stmt"
	"github.com/mdhender/maclo/pkg/l/token"
)

// Build turns the flat list of source lines into the nested tree.
//
// Two things happen here. The five paired constructs are matched with a stack,
// and each closer is folded onto the statement it closes rather than becoming
// a statement of its own. And each line's arguments are read according to the
// grammar of its statement, which is where "the word AD, a group, the word PT"
// becomes an address constant.
//
// Errors accumulate. A line that cannot be built becomes a BadStmt and the
// walk continues, so a malformed statement costs one diagnostic rather than
// the rest of the file.
func Build(f *cst.File) (*Program, token.Errors) {
	b := &builder{}
	p := &Program{Tail: trivia(f.Tail)}

	// stack[0] is the program itself; every opener pushes a frame.
	stack := []*frame{{body: &p.Stmts}}
	cur := func() *frame { return stack[len(stack)-1] }

	for _, line := range f.Lines {
		// A line's own diagnostics are not merged here: cst.Parse already
		// returned them, and reporting them again would double every lexical
		// complaint in the final list.
		if line.Head == stmt.Unknown {
			continue // the cst already said why
		}

		if line.Head.Role() == stmt.RoleClose {
			b.closeFrame(&stack, line)
			continue
		}

		s := b.buildLine(line)
		if s == nil {
			continue
		}
		*cur().body = append(*cur().body, s)
		if body := openerBody(s); body != nil {
			stack = append(stack, &frame{kind: line.Head, opener: s, body: body})
		}
	}

	for _, fr := range stack[1:] {
		b.errs.Add(fr.opener.Pos(), token.StageAST, "%s is never closed", fr.kind)
	}
	return p, b.errs
}

type builder struct {
	errs token.Errors
}

type frame struct {
	kind   stmt.Kind
	opener Stmt
	body   *[]Stmt
}

// openerBody returns where a compound statement's children go, or nil when the
// statement opens nothing. A one-line IF opens nothing: it was already closed
// by the newline that ended it.
func openerBody(s Stmt) *[]Stmt {
	switch t := s.(type) {
	case *Section:
		return &t.Body
	case *BlockDec:
		return &t.Body
	case *Subroutine:
		return &t.Body
	case *LinkRoutine:
		return &t.Body
	case *ChainFrom:
		return &t.Body
	case *If:
		if t.Block {
			return &t.Body
		}
	}
	return nil
}

// closeFrame pops the construct a closing statement ends and folds the
// closer's position, name and label onto the opener.
func (b *builder) closeFrame(stack *[]*frame, line *cst.Line) {
	s := *stack
	if len(s) == 1 {
		b.errs.Add(line.HeadPos, token.StageAST, "%s with nothing open", line.Head)
		return
	}
	fr := s[len(s)-1]
	if !line.Head.Closes(fr.kind) {
		b.errs.Add(line.HeadPos, token.StageAST, "%s closes %s, which was opened at %s",
			line.Head, fr.kind, fr.opener.Pos())
		return
	}
	*stack = s[:len(s)-1]

	label := identOf(line.Label)
	a := b.reader(line)
	var name *Ident
	if line.Head == stmt.EndSect || line.Head == stmt.EndBlock {
		name = a.ident("the name of the " + fr.kind.String())
	}
	a.expectEnd(b, line.Head)

	switch t := fr.opener.(type) {
	case *Section:
		t.EndPos, t.EndName, t.EndLabel = line.HeadPos, name, label
	case *BlockDec:
		t.EndPos, t.EndName, t.EndLabel = line.HeadPos, name, label
	case *Subroutine:
		t.EndPos, t.EndLabel = line.HeadPos, label
	case *LinkRoutine:
		t.EndPos, t.EndLabel = line.HeadPos, label
	case *ChainFrom:
		// The label on an ENDCH is a real, branched-to program label. Four of
		// the eleven chains in the logic of ML/I carry one.
		t.EndPos, t.EndLabel = line.HeadPos, label
	case *If:
		t.EndPos, t.EndLabel = line.HeadPos, label
	}
}

// buildLine turns one source line into a statement.
func (b *builder) buildLine(line *cst.Line) Stmt {
	base := Base{
		Position: line.Pos,
		Label:    identOf(line.Label),
		Lead:     trivia(line.Lead),
	}
	for _, p := range line.Prefixes {
		base.Prefixes = append(base.Prefixes, Prefix{Position: p.Pos, Text: p.Text})
	}
	a := b.reader(line)
	s := b.dispatch(line.Head, line.HeadPos, base, a)
	a.expectEnd(b, line.Head)
	return s
}

func (b *builder) reader(line *cst.Line) *args {
	return &args{b: b, list: line.Args, end: line.EndPos}
}
