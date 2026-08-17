// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ast

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// WriteListing writes the tree as indented L, with the source line of each
// statement in a gutter.
//
// It is a canonical re-render rather than an echo of the input, and that buys
// two things. The indentation is a visible proof of the nesting, so a
// mispaired END shows up as a shape rather than as a subtle difference. And a
// golden diff reports a change in the parse instead of a change in the file's
// whitespace.
func WriteListing(w io.Writer, p *Program) error {
	bw := bufio.NewWriter(w)
	pr := &printer{w: bw, gutter: true}
	pr.program(p)
	return bw.Flush()
}

// WriteSource writes the tree as plain L with no gutter. Feeding its output
// back through the front end must produce the same tree, which is what
// TestRoundTrip checks and what makes the canonical form load bearing rather
// than decorative.
func WriteSource(w io.Writer, p *Program) error {
	bw := bufio.NewWriter(w)
	pr := &printer{w: bw}
	pr.program(p)
	return bw.Flush()
}

type printer struct {
	w      *bufio.Writer
	gutter bool
	depth  int
}

func (p *printer) program(prog *Program) {
	p.stmts(prog.Stmts)
	for _, t := range prog.Tail {
		p.trivia(t)
	}
}

func (p *printer) stmts(list []Stmt) {
	for _, s := range list {
		p.stmt(s)
	}
}

func (p *printer) stmt(s Stmt) {
	b := s.Common()
	for _, t := range b.Lead {
		p.trivia(t)
	}
	p.line(b.Position.Line, head(b)+text(s))

	switch t := s.(type) {
	case *Section:
		p.body(t.Body)
		p.closer(t.EndPos.Line, t.EndLabel, "ENDSECT "+identText(t.EndName))
	case *BlockDec:
		p.body(t.Body)
		p.closer(t.EndPos.Line, t.EndLabel, "ENDBLOCK "+identText(t.EndName))
	case *Subroutine:
		p.body(t.Body)
		p.closer(t.EndPos.Line, t.EndLabel, "ENDSUB")
	case *LinkRoutine:
		p.body(t.Body)
		p.closer(t.EndPos.Line, t.EndLabel, "ENDSUB")
	case *ChainFrom:
		p.body(t.Body)
		p.closer(t.EndPos.Line, t.EndLabel, "ENDCH")
	case *If:
		if t.Block {
			p.body(t.Body)
			p.closer(t.EndPos.Line, t.EndLabel, "END")
		}
	}
}

func (p *printer) body(list []Stmt) {
	p.depth++
	p.stmts(list)
	p.depth--
}

func (p *printer) closer(line int, label *Ident, text string) {
	prefix := ""
	if label != nil {
		prefix = "[" + label.Text + "] "
	}
	p.line(line, prefix+text)
}

func (p *printer) trivia(t Trivia) {
	if t.Kind == Heading {
		p.line(t.Position.Line, "/+"+t.Text+"+/")
		return
	}
	p.line(t.Position.Line, "//"+t.Text+"//")
}

func (p *printer) line(srcLine int, text string) {
	if p.gutter {
		fmt.Fprintf(p.w, "%5d ", srcLine)
	}
	p.w.WriteString(strings.Repeat("  ", p.depth))
	p.w.WriteString(text)
	p.w.WriteByte('\n')
}

// head renders the label and prefixes a statement carries.
//
// The label comes first and the prefixes after it. The source writes them in
// both orders, but nothing depends on which, and a canonical form has to pick
// one.
func head(b *Base) string {
	var sb strings.Builder
	if b.Label != nil {
		sb.WriteString("[" + b.Label.Text + "] ")
	}
	for _, pre := range b.Prefixes {
		sb.WriteString("/-" + pre.Text + "-/ ")
	}
	return sb.String()
}

// text renders one statement without its label, prefixes or body.
func text(s Stmt) string {
	switch t := s.(type) {
	case *PrgStart:
		return "PRGSTART"
	case *PrgEnd:
		return "PRGEND"
	case *Section:
		return "SECTION " + identText(t.Name) + comment(t.Comment, true)

	case *Dec:
		out := "DEC " + identText(t.Name)
		if t.Init != nil {
			out += " INIT " + exprText(t.Init)
		}
		return out
	case *Equate:
		return "EQUATE " + identText(t.Name) + " TO " + identText(t.To)
	case *BlockDec:
		return "BLOCKDEC " + identText(t.Name) + comment(t.Comment, true)

	case *Subroutine:
		out := "SUBROUTINE " + identText(t.Name)
		if t.Param != nil {
			out += "(" + t.Param.Name + ")"
		}
		if t.HasExit {
			out += " EXIT" + comment(t.ExitComment, false)
		}
		return out
	case *LinkRoutine:
		return "LINKROUTINE " + identText(t.Name)
	case *ReturnFrom:
		return "RETURN FROM " + identText(t.Name)
	case *ExitFrom:
		return "EXIT FROM " + identText(t.Name)
	case *LinkBack:
		return "LINK BACK"
	case *Call:
		out := "CALL " + identText(t.Name)
		if t.Arg != nil {
			out += "(" + exprText(t.Arg.Value) + ")" + t.Arg.Type.String()
		}
		if t.Exit != nil {
			out += " EXIT " + identText(t.Exit)
		}
		return out

	case *If:
		out := "IF " + condText(t.Cond) + " THEN"
		if !t.Block && t.Then != nil {
			out += " " + head(t.Then.Common()) + text(t.Then)
		}
		return out
	case *ChainFrom:
		return "CHAIN FROM " + exprText(t.Addr) + " EXIT " + identText(t.Exit)

	case *MoveFrom:
		out := "MOVE FROM " + exprText(t.From) + " TO " + exprText(t.To) + " LENG " + exprText(t.Leng)
		if t.Backwards {
			out += " BACKWARDS"
		}
		return out
	case *MStackFrom:
		return "MSTACK FROM " + exprText(t.From) + " LENG " + exprText(t.Leng) + " ON " + t.On.String()
	case *MUnstackFrom:
		return "MUNSTACK FROM " + exprText(t.From) + " TO " + exprText(t.To) +
			" LENG " + exprText(t.Leng) + " FROM BSTACK"

	case *Read:
		return "READ"
	case *OutputID:
		return "OUTPUTID"
	case *PRText:
		return "PRTEXT" + bracket(t.Text)

	case *Backspace:
		out := "BACKSPACE " + identText(t.Var)
		if t.Giving != nil {
			out += " GIVING " + identText(t.Giving)
		}
		return out
	case *CharMatch:
		out := "CHARMATCH " + identText(t.Ptr)
		for _, arm := range t.Arms {
			out += "," + exprText(arm.Char) + " GOING " + identText(arm.Target)
		}
		return out
	case *GoTo:
		return "GO TO " + identText(t.Target)
	case *Scale:
		out := "SCALE " + identText(t.Var) + " BY " + exprText(t.By)
		if t.Giving != nil {
			out += " GIVING " + identText(t.Giving)
		}
		return out
	case *Set:
		return "SET " + targetsText(t.Targets) + " = " + exprText(t.Value)
	case *SetSW:
		return "SETSW " + targetsText(t.Targets) + " = " + exprText(t.Value)
	case *Stack:
		return "STACK " + stackValsText(t.Values) + " ON " + t.On.String()
	case *Unstack:
		return "UNSTACK " + stackValsText(t.Values) + " FROM BSTACK"
	case *Test:
		out := "TEST " + identText(t.Var) + " GOING "
		for i, l := range t.Targets {
			if i > 0 {
				out += ","
			}
			out += identText(l)
		}
		return out

	case *DC:
		out := "DC "
		for i, e := range t.Args {
			if i > 0 {
				out += ","
			}
			out += exprText(e)
		}
		return out
	case *LayChain:
		return "LAYCHAIN"
	case *HETables:
		return "HETABLES"
	case *OpMac:
		out := "OPMAC " + exprText(t.Name)
		if t.Name2 != nil {
			out += "+" + exprText(t.Name2)
		}
		return out + "," + exprText(t.Dels) + "," + exprText(t.Marker) + "," + exprText(t.Number)

	case *BadStmt:
		return "??" + t.Raw
	}
	return "??"
}

func comment(text string, withComma bool) string {
	if text == "" {
		return ""
	}
	if withComma {
		return ",//" + text + "//"
	}
	return " //" + text + "//"
}

func targetsText(list []Expr) string {
	var parts []string
	for _, e := range list {
		parts = append(parts, exprText(e))
	}
	return strings.Join(parts, ",")
}

func stackValsText(list []*StackVal) string {
	var parts []string
	for _, v := range list {
		parts = append(parts, exprText(v.Value)+"("+v.Type.String()+")")
	}
	return strings.Join(parts, " ")
}

func condText(c *Cond) string {
	if c == nil {
		return "??"
	}
	var parts []string
	for _, r := range c.Rels {
		parts = append(parts, exprText(r.X)+" "+r.Op.String()+" "+exprText(r.Y))
	}
	return strings.Join(parts, " "+c.Join.String()+" ")
}

func identText(i *Ident) string {
	if i == nil {
		return "??"
	}
	return i.Text
}

func bracket(t *TextLit) string {
	if t == nil {
		return "[]"
	}
	return "[" + t.Raw + "]"
}

// exprText renders an expression, adding parentheses only where dropping them
// would change how it reads back.
func exprText(e Expr) string {
	switch t := e.(type) {
	case nil:
		return "??"
	case *Ident:
		if t == nil {
			return "??"
		}
		return t.Text
	case *IntLit:
		if t.Raw != "" {
			return t.Raw
		}
		return strconv.Itoa(t.Value)
	case *CharLit:
		return "'" + t.Raw + "'"
	case *TextLit:
		return "[" + t.Raw + "]"
	case *OF:
		return "OF(" + exprText(t.Arg) + ")"
	case *AD:
		return "AD(" + identText(t.Name) + ")PT"
	case *BlockRef:
		return "BLOCK(" + identText(t.Name) + ")"
	case *Ind:
		return "IND(" + exprText(t.Addr) + ")" + t.Type.String()
	case *RL:
		out := "RL(" + identText(t.Name)
		if t.Adjust != nil {
			out += exprText(t.Adjust)
		}
		return out + ")"
	case *LID:
		return "LID" + bracket(t.Text)
	case *Unary:
		return t.Op.String() + wrap(t.X, precUnary)
	case *Binary:
		p := prec(t.Op)
		sep := ""
		if t.Op == And || t.Op == Or {
			sep = " "
		}
		// The left child may bind as loosely as this node; the right child may
		// not, or "A-(B-C)" would come back as "A-B-C".
		return wrap(t.X, p) + sep + t.Op.String() + sep + wrap(t.Y, p+1)
	case *Bad:
		return "??" + t.Raw
	}
	return "??"
}

const precUnary = 3

func prec(op BinOp) int {
	switch op {
	case And, Or:
		return 0
	case Add, Sub:
		return 1
	case Mul, Div:
		return 2
	}
	return 0
}

// wrap renders a subexpression, parenthesising it when its operator binds more
// loosely than the context needs.
func wrap(e Expr, need int) string {
	if b, ok := e.(*Binary); ok && prec(b.Op) < need {
		return "(" + exprText(e) + ")"
	}
	if u, ok := e.(*Unary); ok && need > precUnary {
		return "(" + exprText(u) + ")"
	}
	return exprText(e)
}
