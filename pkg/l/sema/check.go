// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package sema

import (
	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/stmt"
	"github.com/mdhender/maclo/pkg/l/token"
)

// checkStructure applies the rules that need no symbol table: where a
// statement may appear, which operators are legal where, and whether the
// routine-relative statements name the routine they are in.
//
// It walks with its own context rather than through ast.Inspect, because every
// one of these rules is about where a node sits rather than about the node.
func (c *checker) checkStructure(p *ast.Program) {
	var starts, ends int
	for i, s := range p.Stmts {
		switch s.(type) {
		case *ast.PrgStart:
			starts++
			if i != 0 {
				c.errf(s.Pos(), "PRGSTART is not the first statement (lmap.txt 2.4)")
			}
		case *ast.PrgEnd:
			ends++
			if i != len(p.Stmts)-1 {
				c.errf(s.Pos(), "PRGEND is not the last statement (lmap.txt 2.4)")
			}
		}
		c.stmt(s, stmt.InFrame)
	}
	if starts == 0 {
		c.errf(p.Pos(), "no PRGSTART (lmap.txt 2.4)")
	} else if starts > 1 {
		c.errf(p.Pos(), "%d PRGSTART statements: there is one (lmap.txt 2.4)", starts)
	}
	if ends == 0 {
		c.errf(p.Pos(), "no PRGEND (lmap.txt 2.4)")
	} else if ends > 1 {
		c.errf(p.Pos(), "%d PRGEND statements: there is one (lmap.txt 2.4)", ends)
	}
}

// checker carries where in the program the walk is.
type checker struct {
	tab  *Table
	errs *token.Errors

	section string
	// routine is the enclosing SUBROUTINE or LINKROUTINE, and isLink says
	// which. Both are what RETURN FROM, EXIT FROM and LINK BACK are checked
	// against.
	routine    *ast.Ident
	routineHas bool
	isLink     bool

	ifDepth    int
	chainDepth int
}

func (c *checker) errf(pos token.Position, format string, v ...any) {
	c.errs.Add(pos, token.StageSema, format, v...)
}

func (c *checker) warnf(pos token.Position, format string, v ...any) {
	c.errs.Warn(pos, token.StageSema, format, v...)
}

func (c *checker) stmt(s ast.Stmt, where stmt.Sections) {
	if s == nil {
		return
	}
	if k := s.Kind(); k != stmt.Unknown && k.Sections()&where == 0 {
		c.errf(s.Pos(), "%s may not appear %s: it belongs in %s (%s)",
			k, placeName(where), k.Sections(), k.Doc())
	}
	c.checkNameLength(s)

	switch t := s.(type) {
	case *ast.Section:
		c.section = ""
		if t.Name != nil {
			c.section = t.Name.Text
		}
		class, known := sectionClasses[c.section]
		if !known {
			c.warnf(t.Pos(), "%s is not one of the ten SECTIONs of the logic (lmap.txt 2.4)", c.section)
			class = "program"
		}
		c.matchEndName(t.Name, t.EndName, "SECTION", "ENDSECT")
		for _, in := range t.Body {
			c.stmt(in, sectionMask(class))
		}
		c.section = ""

	case *ast.BlockDec:
		c.matchEndName(t.Name, t.EndName, "BLOCKDEC", "ENDBLOCK")
		for _, in := range t.Body {
			c.stmt(in, where)
		}

	case *ast.Dec:
		c.expr(t.Init, exprCtx{})

	case *ast.Subroutine:
		c.inRoutine(t.Name, t.HasExit, false, t.Body, where)
	case *ast.LinkRoutine:
		c.inRoutine(t.Name, false, true, t.Body, where)

	case *ast.ReturnFrom:
		c.checkRoutineName(t.Pos(), t.Name, "RETURN FROM")
	case *ast.ExitFrom:
		c.checkRoutineName(t.Pos(), t.Name, "EXIT FROM")
		if c.routine != nil && !c.routineHas {
			c.errf(t.Pos(), "EXIT FROM %s: the subroutine declares no EXIT (lmap.txt 4.1.1.3)", identText(t.Name))
		}
	case *ast.LinkBack:
		if !c.isLink {
			c.errf(t.Pos(), "LINK BACK outside a LINKROUTINE (lmap.txt 4.1.2.2)")
		}

	case *ast.Call:
		if t.Arg != nil {
			c.expr(t.Arg.Value, exprCtx{})
		}

	case *ast.If:
		c.cond(t.Cond)
		if t.Block {
			if c.ifDepth > 0 {
				// lmap.txt 4.2.1: "Statements of form b) are never nested
				// within one another." The logic of ML/I obeys it, and an
				// L-map is allowed to rely on it.
				c.warnf(t.Pos(), "a block IF inside a block IF: the manual says they are never nested (lmap.txt 4.2.1)")
			}
			c.ifDepth++
			for _, in := range t.Body {
				c.stmt(in, where)
			}
			c.ifDepth--
			return
		}
		c.stmt(t.Then, where)

	case *ast.ChainFrom:
		c.expr(t.Addr, exprCtx{})
		if c.chainDepth > 0 {
			c.warnf(t.Pos(), "a CHAIN FROM inside a CHAIN FROM (lmap.txt 4.2.2)")
		}
		c.chainDepth++
		for _, in := range t.Body {
			c.stmt(in, where)
		}
		c.chainDepth--

	// The three statements where BLOCK( ) is legal (lmap.txt 4.3).
	case *ast.MoveFrom:
		c.expr(t.From, exprCtx{allowBlock: true})
		c.expr(t.To, exprCtx{allowBlock: true})
		c.expr(t.Leng, exprCtx{allowBlock: true})
	case *ast.MStackFrom:
		c.expr(t.From, exprCtx{allowBlock: true})
		c.expr(t.Leng, exprCtx{allowBlock: true})
	case *ast.MUnstackFrom:
		c.expr(t.From, exprCtx{allowBlock: true})
		c.expr(t.To, exprCtx{allowBlock: true})
		c.expr(t.Leng, exprCtx{allowBlock: true})

	case *ast.CharMatch:
		for _, arm := range t.Arms {
			c.expr(arm.Char, exprCtx{})
		}
	case *ast.Scale:
		c.expr(t.By, exprCtx{})
	case *ast.Set:
		for _, tg := range t.Targets {
			c.expr(tg, exprCtx{})
		}
		c.expr(t.Value, exprCtx{})
	case *ast.SetSW:
		for _, tg := range t.Targets {
			c.expr(tg, exprCtx{})
		}
		// The second form of SETSW ands or ors two operands, and this is the
		// only expression outside a condition where that is allowed.
		c.expr(t.Value, exprCtx{allowLogical: true})
	case *ast.Stack:
		for _, v := range t.Values {
			c.expr(v.Value, exprCtx{})
		}
	case *ast.Unstack:
		for _, v := range t.Values {
			c.expr(v.Value, exprCtx{})
		}

	// The two statements where RL and LID are legal (lmap.txt 6.1).
	case *ast.DC:
		for _, e := range t.Args {
			c.expr(e, exprCtx{allowData: true})
		}
	case *ast.OpMac:
		c.expr(t.Dels, exprCtx{allowData: true})
		c.expr(t.Marker, exprCtx{allowData: true})
		c.expr(t.Number, exprCtx{allowData: true})
	}
}

func (c *checker) inRoutine(name *ast.Ident, hasExit, isLink bool, body []ast.Stmt, where stmt.Sections) {
	savedName, savedHas, savedLink := c.routine, c.routineHas, c.isLink
	c.routine, c.routineHas, c.isLink = name, hasExit, isLink
	for _, in := range body {
		c.stmt(in, where)
	}
	c.routine, c.routineHas, c.isLink = savedName, savedHas, savedLink
}

// checkRoutineName holds the rule that RETURN FROM and EXIT FROM name the
// routine they are written in (lmap.txt 4.1.1.2, 4.1.1.3).
func (c *checker) checkRoutineName(pos token.Position, name *ast.Ident, what string) {
	if name == nil {
		return
	}
	if c.routine == nil {
		c.errf(pos, "%s %s outside a subroutine", what, name.Text)
		return
	}
	if c.routine.Text != name.Text {
		c.errf(pos, "%s %s inside %s: it names the subroutine it is in", what, name.Text, c.routine.Text)
	}
}

func (c *checker) matchEndName(open, close *ast.Ident, opener, closer string) {
	if open == nil || close == nil {
		return
	}
	if open.Text != close.Text {
		c.errf(close.Position, "%s %s closes %s %s, opened at %s",
			closer, close.Text, opener, open.Text, open.Position)
	}
}

// checkNameLength warns about an identifier outside the manual's three to six
// characters (lmap.txt 2.3).
//
// A warning, not an error, and only on the names a program chooses. The rule
// is broken by the language itself - STOPCODE is eight characters, and so are
// two of the ten SECTION names - so enforcing it would reject real L.
func (c *checker) checkNameLength(s ast.Stmt) {
	var names []*ast.Ident
	switch t := s.(type) {
	case *ast.Dec:
		names = append(names, t.Name)
	case *ast.Equate:
		names = append(names, t.Name)
	case *ast.BlockDec:
		names = append(names, t.Name)
	case *ast.Subroutine:
		names = append(names, t.Name)
	case *ast.LinkRoutine:
		names = append(names, t.Name)
	}
	if l := s.Common().Label; l != nil {
		names = append(names, l)
	}
	for _, n := range names {
		if n == nil {
			continue
		}
		if len(n.Text) < 3 || len(n.Text) > 6 {
			c.warnf(n.Position, "%q is %d characters: an identifier of L has three to six (lmap.txt 2.3)",
				n.Text, len(n.Text))
		}
	}
}

// --- expressions -----------------------------------------------------------

// exprCtx says which of the restricted forms are legal in this position.
type exprCtx struct {
	inOF         bool // the argument of OF, where * and / are allowed
	allowLogical bool // the value of a SETSW, where & and | are allowed
	allowBlock   bool // a block moving statement, where BLOCK( ) is allowed
	allowData    bool // a DC or OPMAC argument, where RL and LID are allowed
}

func (c *checker) cond(cd *ast.Cond) {
	if cd == nil {
		return
	}
	for _, r := range cd.Rels {
		c.expr(r.X, exprCtx{})
		c.expr(r.Y, exprCtx{})
	}
}

func (c *checker) expr(e ast.Expr, ctx exprCtx) {
	switch t := e.(type) {
	case nil:
		return
	case *ast.Binary:
		switch t.Op {
		case ast.Mul, ast.Div:
			if !ctx.inOF {
				c.errf(t.Position, "%s is only allowed inside OF( ) (lmap.txt 3.3.1)", t.Op)
			}
		case ast.And, ast.Or:
			if !ctx.allowLogical {
				c.errf(t.Position, "%s is only allowed in SETSW and in the join of a condition", t.Op)
			}
		}
		// Nothing below the top of a SETSW value may be logical.
		inner := ctx
		inner.allowLogical = false
		c.expr(t.X, inner)
		c.expr(t.Y, inner)
	case *ast.Unary:
		inner := ctx
		inner.allowLogical = false
		c.expr(t.X, inner)
	case *ast.OF:
		inner := ctx
		inner.inOF, inner.allowLogical = true, false
		c.expr(t.Arg, inner)
	case *ast.Ind:
		inner := ctx
		inner.allowLogical, inner.allowBlock, inner.allowData = false, false, false
		c.expr(t.Addr, inner)
	case *ast.BlockRef:
		if !ctx.allowBlock {
			c.errf(t.Position, "BLOCK( ) is only allowed in MOVE FROM, MSTACK FROM and MUNSTACK FROM (lmap.txt 4.3)")
		}
	case *ast.RL:
		if !ctx.allowData {
			c.errf(t.Position, "RL( ) is only allowed as an argument of DC or OPMAC (lmap.txt 6.1.1)")
		}
		inner := ctx
		inner.allowLogical = false
		c.expr(t.Adjust, inner)
	case *ast.LID:
		if !ctx.allowData {
			c.errf(t.Position, "LID is only allowed as an argument of DC or OPMAC (lmap.txt 6.1.2)")
		}
	}
}

func sectionMask(class string) stmt.Sections {
	switch class {
	case "vars":
		return stmt.InVARS
	case "data":
		return stmt.InData
	}
	return stmt.InProgram
}

func placeName(where stmt.Sections) string {
	switch where {
	case stmt.InFrame:
		return "outside a SECTION"
	case stmt.InVARS:
		return "in the VARS SECTION"
	case stmt.InData:
		return "in a data SECTION"
	}
	return "in a program SECTION"
}

func identText(i *ast.Ident) string {
	if i == nil {
		return "?"
	}
	return i.Text
}
