// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ast

// Inspect walks the tree in source order, calling f for every node. When f
// returns false the node's children are skipped.
//
// There is one walker rather than one per consumer, so that a statement added
// to stmts.go has exactly one place where forgetting it would hide a subtree
// from both sema and the listing.
func Inspect(n Node, f func(Node) bool) {
	if n == nil || isNilNode(n) {
		return
	}
	if !f(n) {
		return
	}
	switch t := n.(type) {
	// --- layout
	case *Program:
		inspectStmts(t.Stmts, f)
	case *PrgStart, *PrgEnd:
	case *Section:
		Inspect(t.Name, f)
		inspectStmts(t.Body, f)
		Inspect(t.EndName, f)
		Inspect(t.EndLabel, f)

	// --- declarative
	case *Dec:
		Inspect(t.Name, f)
		Inspect(t.Init, f)
	case *Equate:
		Inspect(t.Name, f)
		Inspect(t.To, f)
	case *BlockDec:
		Inspect(t.Name, f)
		inspectStmts(t.Body, f)
		Inspect(t.EndName, f)
		Inspect(t.EndLabel, f)

	// --- routines
	case *Subroutine:
		Inspect(t.Name, f)
		inspectStmts(t.Body, f)
		Inspect(t.EndLabel, f)
	case *LinkRoutine:
		Inspect(t.Name, f)
		inspectStmts(t.Body, f)
		Inspect(t.EndLabel, f)
	case *ReturnFrom:
		Inspect(t.Name, f)
	case *ExitFrom:
		Inspect(t.Name, f)
	case *LinkBack:
	case *Call:
		Inspect(t.Name, f)
		if t.Arg != nil {
			Inspect(t.Arg.Value, f)
		}
		Inspect(t.Exit, f)

	// --- compound
	case *If:
		Inspect(t.Cond, f)
		inspectStmts(t.Stmts(), f)
		Inspect(t.EndLabel, f)
	case *Cond:
		for _, r := range t.Rels {
			Inspect(r, f)
		}
	case *Rel:
		Inspect(t.X, f)
		Inspect(t.Y, f)
	case *ChainFrom:
		Inspect(t.Addr, f)
		Inspect(t.Exit, f)
		inspectStmts(t.Body, f)
		Inspect(t.EndLabel, f)

	// --- block moves
	case *MoveFrom:
		Inspect(t.From, f)
		Inspect(t.To, f)
		Inspect(t.Leng, f)
	case *MStackFrom:
		Inspect(t.From, f)
		Inspect(t.Leng, f)
	case *MUnstackFrom:
		Inspect(t.From, f)
		Inspect(t.To, f)
		Inspect(t.Leng, f)

	// --- input and output
	case *Read, *OutputID:
	case *PRText:
		Inspect(t.Text, f)

	// --- assignment and branching
	case *Backspace:
		Inspect(t.Var, f)
		Inspect(t.Giving, f)
	case *CharMatch:
		Inspect(t.Ptr, f)
		for _, arm := range t.Arms {
			Inspect(arm.Char, f)
			Inspect(arm.Target, f)
		}
	case *GoTo:
		Inspect(t.Target, f)
	case *Scale:
		Inspect(t.Var, f)
		Inspect(t.By, f)
		Inspect(t.Giving, f)
	case *Set:
		inspectExprs(t.Targets, f)
		Inspect(t.Value, f)
	case *SetSW:
		inspectExprs(t.Targets, f)
		Inspect(t.Value, f)
	case *Stack:
		inspectStackVals(t.Values, f)
	case *Unstack:
		inspectStackVals(t.Values, f)
	case *Test:
		Inspect(t.Var, f)
		for _, l := range t.Targets {
			Inspect(l, f)
		}

	// --- the data SECTIONs
	case *DC:
		inspectExprs(t.Args, f)
	case *LayChain, *HETables:
	case *OpMac:
		Inspect(t.Name, f)
		Inspect(t.Name2, f)
		Inspect(t.Dels, f)
		Inspect(t.Marker, f)
		Inspect(t.Number, f)

	// --- expressions
	case *Ident, *IntLit, *CharLit, *TextLit, *Bad:
	case *OF:
		Inspect(t.Arg, f)
	case *AD:
		Inspect(t.Name, f)
	case *BlockRef:
		Inspect(t.Name, f)
	case *Ind:
		Inspect(t.Addr, f)
	case *RL:
		Inspect(t.Name, f)
		Inspect(t.Adjust, f)
	case *LID:
		Inspect(t.Text, f)
	case *Binary:
		Inspect(t.X, f)
		Inspect(t.Y, f)
	case *Unary:
		Inspect(t.X, f)

	case *BadStmt:
	}
}

func inspectStmts(list []Stmt, f func(Node) bool) {
	for _, s := range list {
		Inspect(s, f)
	}
}

func inspectExprs(list []Expr, f func(Node) bool) {
	for _, e := range list {
		Inspect(e, f)
	}
}

func inspectStackVals(list []*StackVal, f func(Node) bool) {
	for _, v := range list {
		Inspect(v.Value, f)
	}
}

// isNilNode reports whether an interface holds a typed nil, which happens
// wherever a builder returned nil for a name it could not read.
func isNilNode(n Node) bool {
	switch t := n.(type) {
	case *Ident:
		return t == nil
	case *IntLit:
		return t == nil
	case *CharLit:
		return t == nil
	case *TextLit:
		return t == nil
	case *Bad:
		return t == nil
	case *OF:
		return t == nil
	case *AD:
		return t == nil
	case *BlockRef:
		return t == nil
	case *Ind:
		return t == nil
	case *RL:
		return t == nil
	case *LID:
		return t == nil
	case *Binary:
		return t == nil
	case *Unary:
		return t == nil
	case *Cond:
		return t == nil
	case *Rel:
		return t == nil
	}
	return false
}
