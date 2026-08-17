// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ast

import (
	"github.com/mdhender/maclo/pkg/l/cst"
	"github.com/mdhender/maclo/pkg/l/stmt"
	"github.com/mdhender/maclo/pkg/l/token"
)

// dispatch reads the arguments of one statement according to its grammar.
//
// The switch is ordered the way the manual's chapters are, so a statement is
// found where the specification puts it. The closers are absent: closeFrame
// handles them.
func (b *builder) dispatch(kind stmt.Kind, pos token.Position, base Base, a *args) Stmt {
	switch kind {
	// --- layout (lmap.txt 2.4)
	case stmt.PrgStart:
		return &PrgStart{Base: base}
	case stmt.PrgEnd:
		return &PrgEnd{Base: base}
	case stmt.Section:
		s := &Section{Base: base, Name: a.ident("the name of the SECTION")}
		s.Comment = a.subsidiaryComment(true)
		return s

	// --- declarative (lmap.txt 5)
	case stmt.Dec:
		s := &Dec{Base: base, Name: a.ident("the name of the variable")}
		if a.takeWord("INIT") {
			s.Init = a.expr()
		}
		return s
	case stmt.Equate:
		s := &Equate{Base: base, Name: a.ident("the new name")}
		a.expectWord("TO")
		s.To = a.ident("the variable it stands for")
		return s
	case stmt.BlockDec:
		s := &BlockDec{Base: base, Name: a.ident("the name of the block")}
		s.Comment = a.subsidiaryComment(true)
		return s

	// --- routines (lmap.txt 4.1)
	case stmt.Subroutine:
		return b.subroutine(base, a)
	case stmt.LinkRoutine:
		return &LinkRoutine{Base: base, Name: a.ident("the name of the linkroutine")}
	case stmt.ReturnFrom:
		return &ReturnFrom{Base: base, Name: a.ident("the name of the subroutine")}
	case stmt.ExitFrom:
		return &ExitFrom{Base: base, Name: a.ident("the name of the subroutine")}
	case stmt.LinkBack:
		return &LinkBack{Base: base}
	case stmt.Call:
		return b.call(base, a)

	// --- compound (lmap.txt 4.2)
	case stmt.If:
		return b.ifStmt(base, a)
	case stmt.ChainFrom:
		s := &ChainFrom{Base: base, Addr: a.expr()}
		a.expectWord("EXIT")
		s.Exit = a.ident("the label to go to when the chain ends")
		return s

	// --- block moves (lmap.txt 4.3)
	case stmt.MoveFrom:
		s := &MoveFrom{Base: base, From: a.expr()}
		a.expectWord("TO")
		s.To = a.expr()
		a.expectWord("LENG")
		s.Leng = a.expr()
		s.Backwards = a.takeWord("BACKWARDS")
		return s
	case stmt.MStackFrom:
		s := &MStackFrom{Base: base, From: a.expr()}
		a.expectWord("LENG")
		s.Leng = a.expr()
		a.expectWord("ON")
		s.On = a.stackName()
		return s
	case stmt.MUnstackFrom:
		s := &MUnstackFrom{Base: base, From: a.expr()}
		a.expectWord("TO")
		s.To = a.expr()
		a.expectWord("LENG")
		s.Leng = a.expr()
		a.expectWord("FROM")
		a.expectWord("BSTACK") // the grammar admits no other stack
		return s

	// --- input and output (lmap.txt 4.4)
	case stmt.Read:
		return &Read{Base: base}
	case stmt.OutputID:
		return &OutputID{Base: base}
	case stmt.PRText:
		return &PRText{Base: base, Text: a.text("the text to print")}

	// --- assignment and branching (lmap.txt 4.5)
	case stmt.Backspace:
		s := &Backspace{Base: base, Var: a.ident("a variable in the SDB block")}
		if a.takeWord("GIVING") {
			s.Giving = a.ident("the variable to receive the value")
		}
		return s
	case stmt.CharMatch:
		return b.charMatch(base, a)
	case stmt.GoTo:
		return &GoTo{Base: base, Target: a.ident("a label")}
	case stmt.Scale:
		s := &Scale{Base: base, Var: a.ident("a variable")}
		a.expectWord("BY")
		s.By = a.expr()
		if a.takeWord("GIVING") {
			s.Giving = a.ident("the variable to receive the result")
		}
		return s
	case stmt.Set:
		s := &Set{Base: base}
		s.Targets, s.Value = b.assignment(a)
		return s
	case stmt.SetSW:
		s := &SetSW{Base: base}
		s.Targets, s.Value = b.assignment(a)
		// The second form ands or ors two operands. Folding it into a Binary
		// keeps SET and SETSW the same shape for a walker.
		if op, ok := a.logicalOp(); ok {
			s.Value = &Binary{Position: s.Value.Pos(), Op: op, X: s.Value, Y: a.expr()}
		}
		return s
	case stmt.Stack:
		s := &Stack{Base: base}
		s.Values = b.stackValues(a, "ON")
		a.expectWord("ON")
		s.On = a.stackName()
		return s
	case stmt.Unstack:
		s := &Unstack{Base: base}
		s.Values = b.stackValues(a, "FROM")
		a.expectWord("FROM")
		a.expectWord("BSTACK")
		return s
	case stmt.Test:
		s := &Test{Base: base, Var: a.ident("a variable")}
		a.expectWord("GOING")
		for {
			s.Targets = append(s.Targets, a.ident("a label"))
			if !a.takePunct(token.Comma) {
				return s
			}
		}

	// --- the data SECTIONs (lmap.txt 6)
	case stmt.DC:
		s := &DC{Base: base}
		for {
			s.Args = append(s.Args, a.expr())
			if !a.takePunct(token.Comma) {
				return s
			}
		}
	case stmt.LayChain:
		return &LayChain{Base: base}
	case stmt.HETables:
		return &HETables{Base: base}
	case stmt.OpMac:
		return b.opMac(base, a)
	}

	b.errs.Add(pos, token.StageAST, "%s is not built yet", kind)
	return &BadStmt{Base: base, Raw: kind.String()}
}

// subroutine reads SUBROUTINE name [(PARxx)] [EXIT [comment]]
// (lmap.txt 4.1.1.1).
func (b *builder) subroutine(base Base, a *args) Stmt {
	s := &Subroutine{Base: base, Name: a.ident("the name of the subroutine")}
	if g := a.peek(); g != nil && g.Kind == cst.ArgGroup {
		a.advance()
		sub := a.sub(g)
		if id := sub.ident("PARPT, PARNM or PARSW"); id != nil {
			dt, ok := DataTypeOf(trimPar(id.Text))
			if !ok || len(id.Text) != 5 || id.Text[:3] != "PAR" {
				a.errf(id.Position, "%q is not a parameter: a subroutine takes PARPT, PARNM or PARSW", id.Text)
			}
			s.Param = &ParamSpec{Position: id.Position, Name: id.Text, Type: dt}
		}
		sub.expectEnd(b, stmt.Subroutine)
	}
	if a.isWord("EXIT") {
		s.ExitPos, s.HasExit = a.pos(), true
		a.advance()
		s.ExitComment = a.subsidiaryComment(false)
	}
	return s
}

func trimPar(s string) string {
	if len(s) == 5 {
		return s[3:]
	}
	return s
}

// call reads CALL name [(value)TYPE] [EXIT label] (lmap.txt 4.1.3).
func (b *builder) call(base Base, a *args) Stmt {
	s := &Call{Base: base, Name: a.ident("the name of the subroutine")}
	if g := a.peek(); g != nil && g.Kind == cst.ArgGroup {
		a.advance()
		arg := &CallArg{Position: g.Pos, Value: a.subExpr(g)}
		arg.Type = a.dataType("the argument")
		s.Arg = arg
	}
	if a.takeWord("EXIT") {
		s.Exit = a.ident("the label to go to on the subroutine's exit")
	}
	return s
}

// ifStmt reads both forms of the conditional (lmap.txt 4.2.1). Block is set
// from one fact: whether anything followed THEN on the line.
func (b *builder) ifStmt(base Base, a *args) Stmt {
	s := &If{Base: base, Cond: b.cond(a)}
	if !a.expectWord("THEN") {
		return s
	}
	if a.atEnd() {
		s.Block = true
		return s
	}
	s.Then = b.inner(a)
	return s
}

// inner reads the statement a one-line IF guards.
//
// It may carry a statement prefix of its own - the logic of ML/I has two of
// them, both "THEN /-OVP-/SET" - which is why the prefix is collected here
// rather than by the line parser.
func (b *builder) inner(a *args) Stmt {
	base := Base{Position: a.pos()}
	for {
		c := a.peek()
		if c == nil || c.Kind != cst.ArgPrefix {
			break
		}
		base.Prefixes = append(base.Prefixes, Prefix{Position: c.Pos, Text: c.Text})
		a.advance()
	}
	base.Position = a.pos()

	first, second := a.peek(), a.at(a.i+1)
	if first == nil || first.Kind != cst.ArgWord {
		a.errf(a.pos(), "expected a statement after THEN, found %s", a.describe())
		return &BadStmt{Base: base}
	}
	words := []string{first.Text}
	if second != nil && second.Kind == cst.ArgWord {
		words = append(words, second.Text)
	}
	kind, consumed, ok := stmt.Lookup(words...)
	if !ok {
		a.errf(first.Pos, "%q is not a statement of L", first.Text)
		return &BadStmt{Base: base, Raw: first.Text}
	}
	if kind.Role() != stmt.RolePlain {
		// lmap.txt 4.2.1 restricts the one-line form to a simple statement.
		a.errf(first.Pos, "%s cannot follow THEN on one line", kind)
	}
	a.i += consumed
	return b.dispatch(kind, first.Pos, base, a)
}

// cond reads the condition of an IF (lmap.txt 4.2.1.1).
func (b *builder) cond(a *args) *Cond {
	c := &Cond{Position: a.pos()}
	c.Rels = append(c.Rels, b.rel(a))
	for {
		op, ok := a.logicalOp()
		if !ok {
			return c
		}
		if len(c.Rels) > 1 && c.Join != op {
			// The two cannot be mixed, and the Cond has one join, so this is
			// reported here rather than in sema.
			a.errf(a.pos(), "a condition joins its relations with all & or all |, not both")
		}
		c.Join = op
		c.Rels = append(c.Rels, b.rel(a))
	}
}

func (b *builder) rel(a *args) *Rel {
	r := &Rel{Position: a.pos(), X: a.expr()}
	switch c := a.peek(); {
	case c == nil:
		a.errf(a.pos(), "expected a comparison")
		return r
	case c.Kind == cst.ArgPunct && c.Punct == token.Equals:
		a.advance()
		r.Op = EQ
	case c.Kind == cst.ArgWord:
		op, ok := RelOpOf(c.Text)
		if !ok {
			a.errf(c.Pos, "expected =, NE, GR, GE or LE, found the word %s", c.Text)
			return r
		}
		a.advance()
		r.Op = op
	default:
		a.errf(c.Pos, "expected =, NE, GR, GE or LE, found %s", a.describe())
		return r
	}
	r.Y = a.expr()
	return r
}

// assignment reads "target (, target)* = value", shared by SET and SETSW.
func (b *builder) assignment(a *args) ([]Expr, Expr) {
	var targets []Expr
	for {
		targets = append(targets, a.target())
		if !a.takePunct(token.Comma) {
			break
		}
	}
	if !a.takePunct(token.Equals) {
		a.errf(a.pos(), "expected =, found %s", a.describe())
		return targets, &Bad{Position: a.pos()}
	}
	return targets, a.expr()
}

// charMatch reads CHARMATCH ptr (, char GOING label)* (lmap.txt 4.5.2).
func (b *builder) charMatch(base Base, a *args) Stmt {
	s := &CharMatch{Base: base, Ptr: a.ident("a pointer variable")}
	for a.takePunct(token.Comma) {
		arm := &CharArm{Position: a.pos(), Char: a.expr()}
		a.expectWord("GOING")
		arm.Target = a.ident("a label")
		s.Arms = append(s.Arms, arm)
	}
	return s
}

// stackValues reads the value-and-type pairs of STACK and UNSTACK up to the
// keyword that ends them.
//
// There is no comma between values and the space before the type tag is
// optional, so the boundary is found by parsing an expression and then taking
// the parenthesised tag that follows it. That works because the expression
// parser will not absorb a group unless a macro name precedes it.
func (b *builder) stackValues(a *args, until string) []*StackVal {
	var out []*StackVal
	for !a.atEnd() && !a.isWord(until) {
		v := &StackVal{Position: a.pos(), Value: a.expr()}
		g := a.group("the type of the value")
		if g == nil {
			// expr made no progress and there is no tag; step over one
			// argument so the loop cannot spin.
			if a.advance() == nil {
				break
			}
			out = append(out, v)
			continue
		}
		sub := a.sub(g)
		v.Type = sub.dataType("the stacked value")
		sub.expectEnd(b, stmt.Stack)
		out = append(out, v)
	}
	return out
}

// opMac reads OPMAC 'name' [+ 'name'] , dels , marker , number
// (lmap.txt 6.2.4).
func (b *builder) opMac(base Base, a *args) Stmt {
	s := &OpMac{Base: base, Name: a.charLit("the name of the operation macro")}
	if a.takePunct(token.Plus) {
		s.Name2 = a.charLit("the second atom of the name")
	}
	a.takePunct(token.Comma)
	s.Dels = a.expr()
	a.takePunct(token.Comma)
	s.Marker = a.expr()
	a.takePunct(token.Comma)
	s.Number = a.expr()
	return s
}
