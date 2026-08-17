// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ast

import (
	"github.com/mdhender/maclo/pkg/l/cst"
	"github.com/mdhender/maclo/pkg/l/stmt"
	"github.com/mdhender/maclo/pkg/l/token"
)

// args reads the argument list of one statement.
//
// It is a cursor over the concrete argument nodes rather than over tokens,
// because the cst has already matched the parentheses. What is left is to say
// what the arguments mean, and that differs per statement.
type args struct {
	b    *builder
	list []*cst.Arg
	i    int
	// end is the position to blame when something is missing off the end of
	// the line.
	end token.Position
}

func (a *args) atEnd() bool { return a.i >= len(a.list) }

func (a *args) peek() *cst.Arg { return a.at(a.i) }

func (a *args) at(i int) *cst.Arg {
	if i < 0 || i >= len(a.list) {
		return nil
	}
	return a.list[i]
}

func (a *args) advance() *cst.Arg {
	c := a.peek()
	if c != nil {
		a.i++
	}
	return c
}

// pos is where the next argument sits, or the end of the line.
func (a *args) pos() token.Position {
	if c := a.peek(); c != nil {
		return c.Pos
	}
	return a.end
}

func (a *args) errf(pos token.Position, format string, v ...any) {
	a.b.errs.Add(pos, token.StageAST, format, v...)
}

// isWord reports whether the next argument is the keyword text.
func (a *args) isWord(text string) bool {
	c := a.peek()
	return c != nil && c.Kind == cst.ArgWord && c.Text == text
}

// takeWord consumes the keyword text if it is next.
func (a *args) takeWord(text string) bool {
	if a.isWord(text) {
		a.i++
		return true
	}
	return false
}

// expectWord consumes the keyword text, reporting if it is not there.
func (a *args) expectWord(text string) bool {
	if a.takeWord(text) {
		return true
	}
	a.errf(a.pos(), "expected %s, found %s", text, a.describe())
	return false
}

func (a *args) isPunct(k token.Kind) bool {
	c := a.peek()
	return c != nil && c.Kind == cst.ArgPunct && c.Punct == k
}

func (a *args) takePunct(k token.Kind) bool {
	if a.isPunct(k) {
		a.i++
		return true
	}
	return false
}

// ident consumes a bare name. what names the thing for the diagnostic.
func (a *args) ident(what string) *Ident {
	c := a.peek()
	if c == nil || c.Kind != cst.ArgWord {
		a.errf(a.pos(), "expected %s, found %s", what, a.describe())
		return nil
	}
	a.i++
	return &Ident{Position: c.Pos, Text: c.Text}
}

// group consumes a parenthesised run.
func (a *args) group(what string) *cst.Arg {
	c := a.peek()
	if c == nil || c.Kind != cst.ArgGroup {
		a.errf(a.pos(), "expected ( %s ), found %s", what, a.describe())
		return nil
	}
	a.i++
	return c
}

// dataType consumes a CH, NM, PT or SW suffix.
func (a *args) dataType(what string) DataType {
	c := a.peek()
	if c != nil && c.Kind == cst.ArgWord {
		if dt, ok := DataTypeOf(c.Text); ok {
			a.i++
			return dt
		}
	}
	a.errf(a.pos(), "expected the type of %s (CH, NM, PT or SW), found %s", what, a.describe())
	return NoType
}

// stackName consumes FSTACK or BSTACK.
func (a *args) stackName() StackName {
	switch {
	case a.takeWord("FSTACK"):
		return FStack
	case a.takeWord("BSTACK"):
		return BStack
	}
	a.errf(a.pos(), "expected FSTACK or BSTACK, found %s", a.describe())
	return FStack
}

// logicalOp consumes & or |.
func (a *args) logicalOp() (BinOp, bool) {
	switch {
	case a.takePunct(token.Amp):
		return And, true
	case a.takePunct(token.Bar):
		return Or, true
	}
	return Add, false
}

// text consumes a bracketed run of characters.
func (a *args) text(what string) *TextLit {
	c := a.peek()
	if c == nil || c.Kind != cst.ArgText {
		a.errf(a.pos(), "expected [ %s ], found %s", what, a.describe())
		return nil
	}
	a.i++
	return &TextLit{Position: c.Pos, Raw: c.Raw, Text: c.Text}
}

// charLit consumes a quote macro.
func (a *args) charLit(what string) *CharLit {
	c := a.peek()
	if c == nil || c.Kind != cst.ArgQuote {
		a.errf(a.pos(), "expected %s in quotes, found %s", what, a.describe())
		return nil
	}
	a.i++
	return &CharLit{Position: c.Pos, Raw: c.Raw, Text: c.Text}
}

// subsidiaryComment reads the // ... // that describes a SECTION, a BLOCKDEC
// or a subroutine's exit (lmap.txt 2.5).
//
// The three are written differently: SECTION and BLOCKDEC put a comma before
// the comment, and SUBROUTINE does not and puts it after EXIT rather than
// after the name. withComma says which shape to expect.
func (a *args) subsidiaryComment(withComma bool) string {
	if withComma && !a.takePunct(token.Comma) {
		return ""
	}
	c := a.peek()
	if c == nil || c.Kind != cst.ArgComment {
		if withComma {
			a.errf(a.pos(), "expected a // comment after the comma, found %s", a.describe())
		}
		return ""
	}
	a.i++
	return c.Text
}

// expectEnd reports anything left over. It is called after every statement,
// and it is the check that turns a gap in this grammar into a diagnostic
// instead of silence.
func (a *args) expectEnd(b *builder, kind stmt.Kind) {
	if a.atEnd() {
		return
	}
	b.errs.Add(a.pos(), token.StageAST, "%s: unexpected %s", kind, a.describe())
}

func (a *args) describe() string {
	c := a.peek()
	if c == nil {
		return "the end of the statement"
	}
	switch c.Kind {
	case cst.ArgWord:
		return "the word " + c.Text
	case cst.ArgNumber:
		return "the number " + c.Raw
	case cst.ArgQuote:
		return "the character '" + c.Raw + "'"
	case cst.ArgText:
		return "the text [" + c.Raw + "]"
	case cst.ArgComment:
		return "a comment"
	case cst.ArgPrefix:
		return "a statement prefix"
	case cst.ArgGroup:
		return "a parenthesised group"
	}
	return "'" + c.Raw + "'"
}

// sub makes a reader over the contents of a group.
func (a *args) sub(g *cst.Arg) *args {
	return &args{b: a.b, list: g.Children, end: g.Pos}
}

// subExpr reads a group that should hold exactly one expression.
func (a *args) subExpr(g *cst.Arg) Expr {
	s := a.sub(g)
	if s.atEnd() {
		a.errf(g.Pos, "empty ( )")
		return &Bad{Position: g.Pos}
	}
	e := s.expr()
	if !s.atEnd() {
		s.errf(s.pos(), "unexpected %s inside ( )", s.describe())
	}
	return e
}

// --- expressions (lmap.txt 3.6) --------------------------------------------

// expr reads an arithmetic expression: an optional leading sign, then terms
// joined by + and -.
func (a *args) expr() Expr {
	pos := a.pos()
	var x Expr
	switch {
	case a.takePunct(token.Minus):
		x = &Unary{Position: pos, Op: Sub, X: a.term()}
	case a.takePunct(token.Plus):
		x = &Unary{Position: pos, Op: Add, X: a.term()}
	default:
		x = a.term()
	}
	for {
		c := a.peek()
		if c == nil || c.Kind != cst.ArgPunct {
			return x
		}
		var op BinOp
		switch c.Punct {
		case token.Plus:
			op = Add
		case token.Minus:
			op = Sub
		default:
			return x
		}
		a.i++
		x = &Binary{Position: c.Pos, Op: op, X: x, Y: a.term()}
	}
}

// term reads a run of primaries joined by * and /. Multiplication is legal
// only inside OF (lmap.txt 3.3.1) and division only defines LICH, but the
// grammar is the same everywhere and sema is what says where it may appear.
func (a *args) term() Expr {
	x := a.primary()
	for {
		c := a.peek()
		if c == nil || c.Kind != cst.ArgPunct {
			return x
		}
		var op BinOp
		switch c.Punct {
		case token.Star:
			op = Mul
		case token.Slash:
			op = Div
		default:
			return x
		}
		a.i++
		x = &Binary{Position: c.Pos, Op: op, X: x, Y: a.primary()}
	}
}

// primary reads one value.
//
// A word followed by a group is a macro call only for the six words that are
// macros. That is what keeps "STACK IDPT (PT)" and "STACK PARSW(SW)" parsing
// the same way: layout is insignificant in L, so the space cannot be what
// decides, and a bare word never swallows the group after it.
func (a *args) primary() Expr {
	c := a.peek()
	if c == nil {
		a.errf(a.pos(), "expected a value")
		return &Bad{Position: a.pos()}
	}
	switch c.Kind {
	case cst.ArgNumber:
		a.i++
		return &IntLit{Position: c.Pos, Value: c.Num, Raw: c.Raw}
	case cst.ArgQuote:
		a.i++
		return &CharLit{Position: c.Pos, Raw: c.Raw, Text: c.Text}
	case cst.ArgText:
		a.i++
		return &TextLit{Position: c.Pos, Raw: c.Raw, Text: c.Text}
	case cst.ArgGroup:
		a.i++
		return a.subExpr(c)
	case cst.ArgWord:
		if e := a.macro(c); e != nil {
			return e
		}
		a.i++
		return &Ident{Position: c.Pos, Text: c.Text}
	}
	a.errf(c.Pos, "expected a value, found %s", a.describe())
	a.i++
	return &Bad{Position: c.Pos, Raw: c.Raw}
}

// macro reads one of the constant-defining macros, or returns nil when the
// word is an ordinary name.
func (a *args) macro(c *cst.Arg) Expr {
	next := a.at(a.i + 1)
	isCall := next != nil && next.Kind == cst.ArgGroup

	switch c.Text {
	case "IND":
		if !isCall {
			return nil
		}
		a.i += 2
		e := &Ind{Position: c.Pos, Addr: a.subExpr(next)}
		e.Type = a.dataType("the indirect address")
		return e

	case "AD":
		if !isCall {
			return nil
		}
		a.i += 2
		e := &AD{Position: c.Pos, Name: a.sub(next).ident("a data label")}
		a.expectWord("PT") // AD is always a pointer (lmap.txt 3.3.3)
		return e

	case "OF":
		if !isCall {
			return nil
		}
		a.i += 2
		return &OF{Position: c.Pos, Arg: a.subExpr(next)}

	case "BLOCK":
		if !isCall {
			return nil
		}
		a.i += 2
		return &BlockRef{Position: c.Pos, Name: a.sub(next).ident("a block name")}

	case "RL":
		if !isCall {
			return nil
		}
		a.i += 2
		return a.relativeLocation(c, next)

	case "LID":
		if next == nil || next.Kind != cst.ArgText {
			return nil
		}
		a.i += 2
		return &LID{Position: c.Pos, Text: &TextLit{Position: next.Pos, Raw: next.Raw, Text: next.Text}}
	}
	return nil
}

// relativeLocation reads RL( label ((+|-) number)* ) (lmap.txt 6.1.1).
func (a *args) relativeLocation(c, g *cst.Arg) Expr {
	s := a.sub(g)
	e := &RL{Position: c.Pos, Name: s.ident("a data label")}
	if !s.atEnd() {
		e.Adjust = s.signedTail()
	}
	if !s.atEnd() {
		s.errf(s.pos(), "unexpected %s inside RL( )", s.describe())
	}
	return e
}

// signedTail reads the run of signed adjustments after a label in RL.
func (a *args) signedTail() Expr {
	var x Expr
	for {
		c := a.peek()
		if c == nil || c.Kind != cst.ArgPunct {
			return x
		}
		var op BinOp
		switch c.Punct {
		case token.Plus:
			op = Add
		case token.Minus:
			op = Sub
		default:
			return x
		}
		a.i++
		y := a.term()
		if x == nil {
			x = &Unary{Position: c.Pos, Op: op, X: y}
		} else {
			x = &Binary{Position: c.Pos, Op: op, X: x, Y: y}
		}
	}
}

// target reads the left hand side of an assignment: a variable or an indirect
// address (lmap.txt 4.5.5).
func (a *args) target() Expr {
	c := a.peek()
	if c != nil && c.Kind == cst.ArgWord && c.Text == "IND" {
		return a.primary()
	}
	if id := a.ident("a variable or IND( )"); id != nil {
		return id
	}
	pos := a.pos()
	a.advance()
	return &Bad{Position: pos}
}

// identOf converts a cst label token.
func identOf(t *token.Token) *Ident {
	if t == nil {
		return nil
	}
	return &Ident{Position: t.Pos, Text: t.Text}
}

// trivia converts the comments written above a statement.
func trivia(toks []token.Token) []Trivia {
	if len(toks) == 0 {
		return nil
	}
	out := make([]Trivia, 0, len(toks))
	for _, t := range toks {
		kind := Note
		if t.Kind == token.Heading {
			kind = Heading
		}
		out = append(out, Trivia{Position: t.Pos, Kind: kind, Text: t.Text})
	}
	return out
}
