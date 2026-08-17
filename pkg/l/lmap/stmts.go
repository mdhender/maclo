// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package lmap

import (
	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/token"
	"github.com/mdhender/maclo/pkg/lowl/op"
)

// body emits the code for one statement, after its label and comments have
// been dealt with.
func (m *mapper) body(s ast.Stmt) {
	m.spilled = 0
	line := s.Pos().Line
	switch t := s.(type) {
	// the VARS SECTION
	case *ast.Dec:
		m.dec(t)
	case *ast.Equate:
		m.equate(t)
	case *ast.BlockDec:
		m.blockDec(t)

	// the data SECTIONs
	case *ast.DC:
		m.dc(t)
	case *ast.LayChain:
		m.layChain(line)
	case *ast.HETables:
		m.heTables(line)
	case *ast.OpMac:
		m.opMac(t)

	// routines
	case *ast.Subroutine:
		m.subroutine(t)
	case *ast.LinkRoutine:
		m.linkRoutine(t)
	case *ast.ReturnFrom:
		m.exitFrom(line, t.Name, m.exits)
	case *ast.ExitFrom:
		m.exitFrom(line, t.Name, 1)
	case *ast.LinkBack:
		m.p.emit(line, op.LINKB)
	case *ast.Call:
		m.call(t)

	// compound
	case *ast.If:
		m.ifStmt(t)
	case *ast.ChainFrom:
		m.chainFrom(t)

	// block moves
	case *ast.MoveFrom:
		m.moveFrom(t)
	case *ast.MStackFrom:
		m.mstackFrom(t)
	case *ast.MUnstackFrom:
		m.munstackFrom(t)

	// input and output
	case *ast.Read:
		m.p.emit(line, op.GOSUB, word(nameRead), num(0))
	case *ast.OutputID:
		m.p.emit(line, op.GOSUB, word(nameOutput), num(0))
	case *ast.PRText:
		m.prText(t)

	// assignment and branching
	case *ast.Backspace:
		m.backspace(t)
	case *ast.CharMatch:
		m.charMatch(t)
	case *ast.GoTo:
		m.goTo(line, t.Target)
	case *ast.Scale:
		m.scale(t)
	case *ast.Set:
		m.set(line, t.Targets, t.Value, false)
	case *ast.SetSW:
		m.set(line, t.Targets, t.Value, true)
	case *ast.Stack:
		m.stack(t)
	case *ast.Unstack:
		m.unstack(t)
	case *ast.Test:
		m.test(t)

	case *ast.BadStmt:
		m.errs.Add(t.Position, token.StageLMap, "this statement did not parse, so it cannot be mapped")
	default:
		m.errs.Add(s.Pos(), token.StageLMap, "%s has no mapping into LOWL", s.Kind())
	}
}

// endsWithTransfer reports whether the last thing emitted leaves for good.
//
// It is asked at the end of a subroutine, to decide whether a return has to be
// added. A conditional branch does not count and neither does the entry of a
// jump table, which is what the fourth operand is checked for: a GO with C or
// T on it is taken only when the exit it stands for was the one taken.
func (p *Program) endsWithTransfer() bool {
	for i := len(p.Stmts) - 1; i >= 0; i-- {
		s := p.Stmts[i]
		switch s.Op {
		case "":
			if s.Label != "" {
				// something can arrive here, so the words before it say
				// nothing about whether the next word is reachable
				return false
			}
			continue
		case "NB":
			continue
		case "EXIT", "LINKB":
			return true
		case "GO":
			return len(s.Args) == 4 && s.Args[3].Text == "X"
		}
		return false
	}
	return false
}

// subroutine emits SUBROUTINE ... ENDSUB.
//
// The exit count is one more than the number of exit labels, because the
// normal return is itself an exit: a call is followed by one jump table entry
// per exit label, and returning past the last of them is what "return" means.
// So RETURN FROM is the highest numbered exit and EXIT FROM is the first, and
// pkg/lowl/assembler checks both against the count.
func (m *mapper) subroutine(t *ast.Subroutine) {
	if t.Name == nil {
		return
	}
	m.checkName(t.Name)
	m.sub, m.exits = t.Name.Text, 1
	if t.HasExit {
		m.exits = 2
	}
	param := "X"
	if t.Param != nil {
		// LOWL stores the argument in PARNM whatever L called it; the other
		// two names are equated to it in the declarations
		param = "PARNM"
	}
	m.p.comment(t.ExitPos.Line, t.ExitComment)
	m.p.emit(t.Position.Line, op.SUBR, word(t.Name.Text), word(param), num(m.exits))
	m.stmts(t.Body)
	if t.EndLabel != nil {
		m.p.label(t.EndLabel.Text)
	}
	if !m.p.endsWithTransfer() {
		// The manual does not say what running off the end of a subroutine
		// means, and one subroutine of ML/I does it -- LULAYK ends with a
		// chain that cannot be exhausted. Returning is the reading that costs
		// nothing: the alternative is to fall into whatever was emitted next.
		m.p.emit(t.EndPos.Line, op.EXIT, num(m.exits), word(t.Name.Text))
	}
	m.sub, m.exits = "", 0
	m.p.blank()
}

// linkRoutine emits LINKROUTINE ... ENDSUB.
//
// A linkroutine returns through a variable rather than through the stack of
// return addresses, because the code that returns from it is not the code that
// was called: control leaves through a GO TO into the main logic and comes
// back to a label inside the routine much later.
func (m *mapper) linkRoutine(t *ast.LinkRoutine) {
	if t.Name == nil {
		return
	}
	m.checkName(t.Name)
	m.p.emit(t.Position.Line, op.LINKR, word(t.Name.Text))
	m.stmts(t.Body)
	if t.EndLabel != nil {
		m.p.label(t.EndLabel.Text)
	}
	m.p.blank()
}

// exitFrom emits RETURN FROM and EXIT FROM, which differ only in the number.
func (m *mapper) exitFrom(line int, name *ast.Ident, n int) {
	if name == nil {
		return
	}
	if m.sub == "" {
		m.errs.Add(token.Position{Line: line}, token.StageLMap, "%s is left from outside any subroutine", name.Text)
		return
	}
	m.p.emit(line, op.EXIT, num(n), word(m.sub))
}

// call emits CALL, which is a GOSUB and, when the call has an exit label, the
// one entry jump table that follows it.
func (m *mapper) call(t *ast.Call) {
	if t.Name == nil {
		return
	}
	line := t.Position.Line
	if m.mdCall(t) {
		return
	}
	if t.Arg != nil {
		m.load(line, t.Arg.Value)
	}
	m.p.emit(line, op.GOSUB, word(t.Name.Text), num(0))
	m.exitOf(line, t.Exit)
}

// exitOf emits the jump table entry a call's exit label needs. Exit one is the
// first entry after the GOSUB, which is what EXIT FROM sets.
func (m *mapper) exitOf(line int, exit *ast.Ident) {
	if exit == nil {
		return
	}
	m.p.emit(line, op.GO, word(m.labelName(exit.Text)), num(0), word("X"), word("C"))
}

// mdCall emits the calls that are not calls: the machine dependent
// subroutines, which chapter 7 of the manual leaves to the implementor.
//
// Four of them are here rather than in the prelude because LOWL already has
// them. MDTEST asks whether a character can be part of an atom, which is what
// GOPC branches on, and the manual says in so many words that it is highly
// desirable to replace it with in-line code. MDCONV, MDFIND and MDOP are
// instructions of this machine. The rest are subroutines the prelude supplies.
func (m *mapper) mdCall(t *ast.Call) bool {
	line := t.Position.Line
	switch t.Name.Text {
	case "MDTEST":
		if t.Arg == nil || t.Exit == nil {
			m.errs.Add(t.Position, token.StageLMap, "MDTEST wants a pointer and an exit label")
			return true
		}
		m.p.emit(line, op.LCI, word(m.address(line, t.Arg.Value)), word("X"))
		m.p.emit(line, op.GOPC, word(m.labelName(t.Exit.Text)), num(0), word("X"), word("X"))
	case "MDNUM":
		m.p.emit(line, op.GOSUB, word(nameNumber), num(0))
		m.exitOf(line, t.Exit)
	case "MDERPR":
		m.p.emit(line, op.GOSUB, word(namePrintText), num(0))
	case "MDQUOT":
		m.p.emit(line, op.LCN, word("QUTREP"))
		m.p.emit(line, op.GOSUB, word("MDERCH"), word("X"))
	case "MDCONV", "MDFIND":
		m.p.emit(line, op.GOSUB, word(t.Name.Text), word("X"))
	case "MDOP":
		// The manual writes MDOP's action out as L, and its first line is a
		// test for a zero divisor that goes to the overflow label. This
		// machine does the arithmetic and takes its first exit for that case,
		// so the branch is all that is left to emit.
		m.p.emit(line, op.GOSUB, word("MDOP"), word("X"))
		m.p.emit(line, op.GO, word(overflowLabel), num(0), word("X"), word("C"))
	case "MDINIT":
		// "In most implementations MDINIT will be null", and in this one it is
		m.p.comment(line, "MDINIT is null in this L-map")
	default:
		return false
	}
	return true
}

// ifStmt emits IF, in either of its forms.
//
// There is one emitter because there is one statement: whether the guarded
// code was written after THEN or on the lines below it changes where the join
// label goes and nothing else. The case worth separating is the other one --
// the manual says most L-maps will want to recognise THEN GO TO, and two
// thirds of the IFs in ML/I are that shape -- because a condition that is
// already a branch needs no label at all.
func (m *mapper) ifStmt(t *ast.If) {
	line := t.Position.Line
	if target, ok := thenGoTo(t); ok {
		m.condition(line, t.Cond, m.labelName(target.Text), true)
		return
	}
	join := m.p.newLabel("LOIF")
	m.condition(line, t.Cond, join, false)
	m.stmts(t.Stmts())
	if t.EndLabel != nil {
		m.p.label(t.EndLabel.Text)
	}
	m.p.label(join)
}

// thenGoTo recognises the one-line IF whose guarded statement is a branch and
// nothing else. A label, a prefix or a comment on the guarded statement puts
// it back in the general case, because each of those wants a place of its own.
func thenGoTo(t *ast.If) (*ast.Ident, bool) {
	if t.Block || t.Then == nil {
		return nil, false
	}
	g, ok := t.Then.(*ast.GoTo)
	if !ok || g.Target == nil {
		return nil, false
	}
	b := g.Common()
	if b.Label != nil || len(b.Prefixes) != 0 || len(b.Lead) != 0 {
		return nil, false
	}
	return g.Target, true
}

// condition emits a condition and a branch to target, taken when the condition
// holds if want is true and when it fails if want is false.
//
// The two joining operators are the reason this is not one line per relation.
// A condition is either a run of relations all of which must hold or a run any
// of which may -- the manual forbids mixing them -- so one of the two readings
// always needs a label of its own to fall past.
func (m *mapper) condition(line int, c *ast.Cond, target string, want bool) {
	if c == nil || len(c.Rels) == 0 {
		m.errs.Add(token.Position{Line: line}, token.StageLMap, "this condition is empty")
		return
	}
	if len(c.Rels) == 1 {
		m.relation(line, c.Rels[0], target, want)
		return
	}
	all := c.Join == ast.And
	// The straightforward reading: every relation branches to the target on
	// its own. It is right for "all of these, and branch when one fails" and
	// for "any of these, and branch when one holds".
	if all != want {
		for _, r := range c.Rels {
			m.relation(line, r, target, want)
		}
		return
	}
	// The other reading needs somewhere to give up. All but the last relation
	// branches past the branch, and the last one decides.
	past := m.p.newLabel("LOCND")
	for _, r := range c.Rels[:len(c.Rels)-1] {
		m.relation(line, r, past, !want)
	}
	m.relation(line, c.Rels[len(c.Rels)-1], target, want)
	m.p.label(past)
}

// relation emits one comparison and the branch that follows it.
func (m *mapper) relation(line int, r *ast.Rel, target string, want bool) {
	if r == nil {
		return
	}
	m.compare(line, r)
	m.p.emit(line, branchFor(r.Op, want), word(target), num(0), word("X"), word("X"))
}

// branchFor is the branch that goes when a relation holds, or the one that
// goes when it does not.
//
// L has no "less than": the manual's five operators are =, NE, GR, GE and LE.
// LOWL has six, and the missing one is exactly what the negation of GE needs,
// which is why nothing here has to rewrite a comparison to invert it.
func branchFor(rel ast.RelOp, want bool) op.Code {
	if want {
		switch rel {
		case ast.EQ:
			return op.GOEQ
		case ast.NE:
			return op.GONE
		case ast.GR:
			return op.GOGR
		case ast.GE:
			return op.GOGE
		case ast.LE:
			return op.GOLE
		}
		return op.GOEQ
	}
	switch rel {
	case ast.EQ:
		return op.GONE
	case ast.NE:
		return op.GOEQ
	case ast.GR:
		return op.GOLE
	case ast.GE:
		return op.GOLT
	case ast.LE:
		return op.GOGR
	}
	return op.GONE
}

// compare emits the comparison of one relation, leaving the answer in the
// machine's condition.
//
// Characters and numbers are compared in different registers, and which is
// wanted is read off the left hand side: an indirect load written with the CH
// data type is a character and everything else is a number or a pointer. The
// manual says the same thing -- "the data types to be compared can be found by
// examining the last two characters of the first argument" -- and the
// restriction that makes it safe is that a constant never appears on the left.
func (m *mapper) compare(line int, r *ast.Rel) {
	if ind, ok := r.X.(*ast.Ind); ok && ind.Type == ast.CH {
		m.p.emit(line, op.LCI, word(m.address(line, ind.Addr)), word("X"))
		m.compareChar(line, r.Y)
		return
	}
	m.load(line, r.X)
	m.compareValue(line, r.X, r.Y)
}

// compareChar compares the character register with the right hand side.
func (m *mapper) compareChar(line int, y ast.Expr) {
	switch t := y.(type) {
	case *ast.CharLit:
		if isNamedChar(t) {
			m.p.emit(line, op.CCN, m.charArg(t))
			return
		}
		m.p.emit(line, op.CCL, m.charArg(t))
	case *ast.Ind:
		m.p.emit(line, op.CCI, word(m.address(line, t.Addr)))
	case *ast.Ident:
		if t.Text == "STOPCODE" {
			// the one character constant the assembler names for itself: it
			// stands for the end of the input, so it has to be a code no
			// character can have
			m.p.emit(line, op.CCN, word("STOPCD"))
			return
		}
		if o, ok := m.operandOf(t); ok && o.isVar() {
			m.p.emit(line, op.CCN, word(o.name))
			return
		}
		m.p.emit(line, op.CCN, m.mustLiteral(t))
	default:
		m.errs.Add(y.Pos(), token.StageLMap, "a character can only be compared with a character")
	}
}

// compareValue compares the accumulator with the right hand side.
func (m *mapper) compareValue(line int, x, y ast.Expr) {
	flag := "X"
	if typeOf(x) == ast.PT {
		// an unsigned address rather than a signed number
		flag = "A"
	}
	if o, ok := m.operandOf(y); ok {
		if o.isVar() {
			m.p.emit(line, op.CAV, word(o.name), word(flag))
			return
		}
		m.p.emit(line, op.CAL, o.lit)
		return
	}
	if ind, ok := y.(*ast.Ind); ok {
		m.p.emit(line, op.CAI, word(m.address(line, ind.Addr)), word(flag))
		return
	}
	// anything else has to be worked out first, and the accumulator is busy
	spill := m.spill(line)
	m.p.emit(line, op.STV, word(spill), word("X"))
	m.load(line, y)
	m.p.emit(line, op.STV, word(nameTemp), word("X"))
	m.p.emit(line, op.LAV, word(spill), word("X"))
	m.p.emit(line, op.CAV, word(nameTemp), word(flag))
}

// typeOf is the data type of an expression, as far as the L-map cares: it is
// used to choose between an address comparison and a signed one, which this
// machine does not distinguish anyway.
func typeOf(e ast.Expr) ast.DataType {
	switch t := e.(type) {
	case *ast.Ident:
		return ast.TypeOfName(t.Text)
	case *ast.Ind:
		return t.Type
	case *ast.AD, *ast.BlockRef:
		return ast.PT
	case *ast.Binary:
		return typeOf(t.X)
	case *ast.Unary:
		return typeOf(t.X)
	}
	return ast.NM
}

// mustLiteral is operandOf where only a literal will do.
func (m *mapper) mustLiteral(e ast.Expr) Arg {
	if o, ok := m.operandOf(e); ok && !o.isVar() {
		return o.lit
	}
	m.errs.Add(e.Pos(), token.StageLMap, "a constant is wanted here")
	return num(0)
}

// chainFrom emits CHAIN FROM ... ENDCH.
//
// The chain walking itself is the MD-logic's: LOSCHN loads the first link and
// LOECHN the next, and each says through its exits whether there was one. So
// the statement is the two calls, the loop back, and the exit label the L
// source named for the case where the chain was empty to begin with.
func (m *mapper) chainFrom(t *ast.ChainFrom) {
	line := t.Position.Line
	if t.Exit == nil {
		m.errs.Add(t.Position, token.StageLMap, "CHAIN FROM wants an exit label")
		return
	}
	m.load(line, t.Addr)
	m.p.emit(line, op.GOSUB, word(nameStartChain), num(0))
	m.p.emit(line, op.GO, word(m.labelName(t.Exit.Text)), num(0), word("X"), word("C"))
	loop := m.p.newLabel("LOCH")
	m.p.label(loop)
	m.stmts(t.Body)
	if t.EndLabel != nil {
		m.p.label(t.EndLabel.Text)
	}
	m.p.emit(t.EndPos.Line, op.GOSUB, word(nameEndChain), num(0))
	m.p.emit(t.EndPos.Line, op.GO, word(loop), num(0), word("X"), word("C"))
}

// goTo emits an unconditional branch.
func (m *mapper) goTo(line int, target *ast.Ident) {
	if target == nil {
		return
	}
	m.p.emit(line, op.GO, word(m.labelName(target.Text)), num(0), word("X"), word("X"))
}

// test emits the multi-way branch: the value selects one of the labels, the
// first of them standing for zero.
func (m *mapper) test(t *ast.Test) {
	line := t.Position.Line
	if t.Var == nil {
		return
	}
	m.p.emit(line, op.GOADD, word(t.Var.Text))
	for _, target := range t.Targets {
		if target == nil {
			continue
		}
		m.p.emit(line, op.GO, word(m.labelName(target.Text)), num(0), word("X"), word("T"))
	}
}

// charMatch emits the run of comparisons the manual writes the statement's
// action out as: one IF per arm, all of them against the same character.
func (m *mapper) charMatch(t *ast.CharMatch) {
	line := t.Position.Line
	if t.Ptr == nil {
		return
	}
	for i, arm := range t.Arms {
		if arm == nil || arm.Target == nil {
			continue
		}
		flag := "X"
		if i > 0 {
			// the same load as the one before it, which the machine is free
			// to skip
			flag = "R"
		}
		m.p.emit(line, op.LCI, word(t.Ptr.Text), word(flag))
		m.compareChar(line, arm.Char)
		m.p.emit(line, op.GOEQ, word(m.labelName(arm.Target.Text)), num(0), word("X"), word("X"))
	}
}

// set emits SET and SETSW, which differ only in that a switch may be masked.
//
// The addresses of any indirect targets are worked out first, because
// evaluating the value needs the accumulator and would lose them. Every store
// but the last says the accumulator must be preserved, which is how one value
// reaches several places.
func (m *mapper) set(line int, targets []ast.Expr, value ast.Expr, logical bool) {
	dest := make([]string, len(targets))
	for i, target := range targets {
		if ind, ok := target.(*ast.Ind); ok {
			dest[i] = m.storeAddress(line, ind.Addr)
		}
	}

	if logical {
		m.logical(line, value)
	} else {
		m.load(line, value)
	}

	for i, target := range targets {
		flag := "P"
		if i == len(targets)-1 {
			flag = "X"
		}
		switch t := target.(type) {
		case *ast.Ident:
			m.checkName(t)
			m.p.emit(line, op.STV, word(t.Text), word(flag))
		case *ast.Ind:
			m.p.emit(line, op.STI, word(dest[i]), word(flag))
		default:
			m.errs.Add(target.Pos(), token.StageLMap, "this cannot be assigned to")
		}
	}
}

// logical evaluates the right hand side of a SETSW, where the two bitwise
// operators are allowed.
func (m *mapper) logical(line int, value ast.Expr) {
	b, ok := value.(*ast.Binary)
	if !ok || (b.Op != ast.And && b.Op != ast.Or) {
		m.load(line, value)
		return
	}
	m.load(line, b.X)
	o, isOperand := m.operandOf(b.Y)
	switch {
	case b.Op == ast.And && isOperand && o.isVar():
		m.p.emit(line, op.ANDV, word(o.name))
	case b.Op == ast.And && isOperand:
		m.p.emit(line, op.ANDL, o.lit)
	case b.Op == ast.Or && isOperand && !o.isVar():
		m.p.emit(line, op.ORL, o.lit)
	case b.Op == ast.Or:
		// LOWL can or a literal into the accumulator and nothing else
		m.errs.Add(b.Position, token.StageLMap, "LOWL has no way to or a variable into the accumulator")
	default:
		m.errs.Add(b.Position, token.StageLMap, "the right of %s has to be a variable or a constant", b.Op)
	}
}

// scale emits the one multiplication L has. LOWL multiplies the accumulator by
// a literal, so the factor has to be a constant, which the manual requires of
// SCALE anyway.
func (m *mapper) scale(t *ast.Scale) {
	line := t.Position.Line
	if t.Var == nil {
		return
	}
	m.p.emit(line, op.LAV, word(t.Var.Text), word("X"))
	m.p.emit(line, op.MULTL, m.mustLiteral(t.By))
	target := t.Var
	if t.Giving != nil {
		target = t.Giving
	}
	m.p.emit(line, op.STV, word(target.Text), word("X"))
}

// backspace reads a variable's former value off the backwards stack.
//
// The manual defines it by rewriting: the stacked copy of the block is at
// DBUGPT, so the value wanted is the one N words in, where N is how far the
// variable is into the block. That is the offset the declarations were walked
// for. The form without GIVING leaves the address rather than the value, and
// the manual allows it to clobber TEMPT, which is where it leaves it.
func (m *mapper) backspace(t *ast.Backspace) {
	line := t.Position.Line
	if t.Var == nil {
		return
	}
	n, ok := m.slot[t.Var.Text]
	if !ok {
		m.errs.Add(t.Var.Position, token.StageLMap, "%s is not in a block, so it cannot be backspaced", t.Var.Text)
		return
	}
	offset := ofArg(itoa(n) + "*LNM")
	if t.Giving == nil {
		m.p.emit(line, op.LAV, word(stackedBlock), word("X"))
		m.p.emit(line, op.AAL, offset)
		m.p.emit(line, op.STV, word(addressLeftIn), word("X"))
		return
	}
	m.p.emit(line, op.LBV, word(stackedBlock))
	m.p.emit(line, op.LAM, offset)
	m.p.emit(line, op.STV, word(t.Giving.Text), word("X"))
}

// stack pushes values, one instruction each.
func (m *mapper) stack(t *ast.Stack) {
	push := op.FSTK
	if t.On == ast.BStack {
		push = op.BSTK
	}
	for _, v := range t.Values {
		if v == nil {
			continue
		}
		line := v.Position.Line
		if v.Type == ast.CH {
			m.p.emit(line, op.LCI, word(m.address(line, v.Value)), word("X"))
			m.p.emit(line, op.CFSTK)
			continue
		}
		m.load(line, v.Value)
		m.p.emit(line, push)
	}
}

// unstack pops values. Only the backwards stack can be unstacked, which is why
// there is no direction to read.
func (m *mapper) unstack(t *ast.Unstack) {
	for _, v := range t.Values {
		if v == nil {
			continue
		}
		id, ok := v.Value.(*ast.Ident)
		if !ok {
			m.errs.Add(v.Value.Pos(), token.StageLMap, "only a variable can be unstacked into")
			continue
		}
		m.p.emit(v.Position.Line, op.UNSTK, word(id.Text))
	}
}

// prText emits a literal string on the error message stream.
//
// The raw text is used rather than the decoded one because LOWL spells a
// newline inside a message the same way L does, with a dollar sign, and MESS
// is where it is turned back into a newline.
func (m *mapper) prText(t *ast.PRText) {
	if t.Text == nil || t.Text.Raw == "" {
		return
	}
	m.p.emit(t.Position.Line, op.MESS, quoted(t.Text.Raw))
}

// itoa keeps strconv out of the statement emitters, which write a lot of
// small numbers and nothing else.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
