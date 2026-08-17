// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package sema

import (
	"sort"

	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/token"
)

// Check applies the structural rules and then resolves every name.
//
// The four passes are separate on purpose. The structural rules need no
// symbols; collection has to finish before references can be looked at,
// because L lets a name be used before it is declared anywhere - a GO TO in
// one SECTION reaching a label in another is ordinary; and the report comes
// last so an undefined name is named once with all its uses, rather than once
// per use. That last is the behaviour of pkg/lowl/assembler, which gathers its
// undefined symbols and prints them after the pass.
func Check(p *ast.Program) (*Table, token.Errors) {
	tab := NewTable()
	var errs token.Errors

	c := &checker{tab: tab, errs: &errs}
	c.checkStructure(p)

	r := &resolver{tab: tab, errs: &errs}
	r.seed(p)
	r.collect(p)
	r.reference(p)
	r.report()

	return tab, errs
}

type resolver struct {
	tab  *Table
	errs *token.Errors

	section string
	// inData says whether the walk is in MACNAMES or DELS, which is what
	// decides whether a label is a data label or a program label.
	inData bool
	block  string
}

func (r *resolver) errf(pos token.Position, format string, v ...any) {
	r.errs.Add(pos, token.StageSema, format, v...)
}

func (r *resolver) warnf(pos token.Position, format string, v ...any) {
	r.errs.Warn(pos, token.StageSema, format, v...)
}

// --- pass 0: the names L defines for itself --------------------------------

func (r *resolver) seed(p *ast.Program) {
	add := func(name string, kind Kind) *Symbol {
		s := &Symbol{Name: name, Kind: kind, Predefined: true, Type: ast.TypeOfName(name)}
		got, _ := r.tab.insert(s)
		return got
	}
	for _, c := range predefinedConstants {
		add(c.Name, Constant)
	}
	// Sorted, because a map's iteration order is random and the symbol
	// listing is compared byte for byte.
	for _, name := range sortedKeys(lengthMacros) {
		add(name, Constant)
	}
	for _, name := range sortedKeys(mdLabels) {
		add(name, MDLabel)
	}
	for _, name := range sortedKeys(mdSubs) {
		sub := mdSubs[name]
		s := add(name, MDSub)
		s.Param, s.HasExit = sub.Param, sub.HasExit
	}

	// HETABLES generates three data labels that appear nowhere in the source
	// (lmap.txt 6.2.3.1). Seeding them only when the statement is present is
	// what keeps the allowance checked rather than silent.
	var hasHETables bool
	ast.Inspect(p, func(n ast.Node) bool {
		if _, ok := n.(*ast.HETables); ok {
			hasHETables = true
		}
		return true
	})
	if hasHETables {
		for _, name := range heTableLabels {
			add(name, DataLabel)
		}
	}
}

// --- pass 1: every declaration ---------------------------------------------

func (r *resolver) collect(p *ast.Program) {
	for _, s := range p.Stmts {
		r.collectStmt(s)
	}
}

func (r *resolver) collectStmt(s ast.Stmt) {
	if s == nil {
		return
	}
	if l := s.Common().Label; l != nil {
		kind := ProgLabel
		if r.inData {
			kind = DataLabel
		}
		r.declare(l, l.Text, kind, l.Position)
	}

	switch t := s.(type) {
	case *ast.Section:
		if t.Name != nil {
			r.declare(t.Name, t.Name.Text, SectionName, t.Name.Position)
			r.section = t.Name.Text
			r.inData = sectionClasses[t.Name.Text] == "data"
		}
		for _, in := range t.Body {
			r.collectStmt(in)
		}
		// A label on the closing statement is a label like any other.
		r.collectCloserLabel(t.EndLabel)
		r.section, r.inData = "", false

	case *ast.BlockDec:
		if t.Name != nil {
			r.declare(t.Name, t.Name.Text, BlockName, t.Name.Position)
			// Each block defines a constant giving its size
			// (lmap.txt 3.3.4.4). Deriving it covers the one the manual's
			// list omits.
			r.declare(nil, blockSizeName(t.Name.Text), Constant, t.Name.Position)
		}
		saved := r.block
		if t.Name != nil {
			r.block = t.Name.Text
		}
		for _, in := range t.Body {
			r.collectStmt(in)
		}
		r.block = saved
		r.collectCloserLabel(t.EndLabel)

	case *ast.Dec:
		if t.Name != nil {
			sym, _ := r.declare(t.Name, t.Name.Text, Variable, t.Name.Position)
			if sym != nil {
				sym.Section, sym.Block = r.section, r.block
			}
		}
	case *ast.Equate:
		if t.Name != nil {
			sym, _ := r.declare(t.Name, t.Name.Text, Variable, t.Name.Position)
			if sym != nil {
				sym.Section, sym.Block = r.section, r.block
			}
		}

	case *ast.Subroutine:
		if t.Name != nil {
			sym, _ := r.declare(t.Name, t.Name.Text, SubName, t.Name.Position)
			if sym != nil {
				sym.Param, sym.HasExit, sym.Section = t.Param, t.HasExit, r.section
			}
		}
		for _, in := range t.Body {
			r.collectStmt(in)
		}
		r.collectCloserLabel(t.EndLabel)

	case *ast.LinkRoutine:
		if t.Name != nil {
			sym, _ := r.declare(t.Name, t.Name.Text, LinkName, t.Name.Position)
			if sym != nil {
				sym.Section = r.section
			}
		}
		for _, in := range t.Body {
			r.collectStmt(in)
		}
		r.collectCloserLabel(t.EndLabel)

	case *ast.ChainFrom:
		for _, in := range t.Body {
			r.collectStmt(in)
		}
		// The label on an ENDCH is a program label, and in the L source of
		// ML/I one of them is the exit of a CALL. Missing it would make four
		// labels vanish and four uses of them undefined.
		r.collectCloserLabel(t.EndLabel)

	case *ast.If:
		for _, in := range t.Stmts() {
			r.collectStmt(in)
		}
		r.collectCloserLabel(t.EndLabel)
	}
}

func (r *resolver) collectCloserLabel(l *ast.Ident) {
	if l == nil {
		return
	}
	kind := ProgLabel
	if r.inData {
		kind = DataLabel
	}
	r.declare(l, l.Text, kind, l.Position)
}

func (r *resolver) declare(node ast.Node, name string, kind Kind, pos token.Position) (*Symbol, bool) {
	sym, ok := r.tab.define(node, name, kind, pos)
	if !ok {
		where := "predefined by the language"
		if sym.Def.IsValid() {
			where = "already declared at " + sym.Def.String()
		}
		r.errf(pos, "%s is %s", name, where)
		return sym, false
	}
	return sym, true
}

// --- pass 2: every reference -----------------------------------------------

func (r *resolver) reference(p *ast.Program) {
	for _, s := range p.Stmts {
		r.referenceStmt(s)
	}
}

func (r *resolver) referenceStmt(s ast.Stmt) {
	if s == nil {
		return
	}
	switch t := s.(type) {
	case *ast.Section:
		if t.EndName != nil {
			r.tab.use(t.EndName, t.EndName.Text, AsEndName, t.EndName.Position)
		}
		for _, in := range t.Body {
			r.referenceStmt(in)
		}
	case *ast.BlockDec:
		if t.EndName != nil {
			r.tab.use(t.EndName, t.EndName.Text, AsEndName, t.EndName.Position)
		}
		for _, in := range t.Body {
			r.referenceStmt(in)
		}

	case *ast.Dec:
		r.value(t.Init)
	case *ast.Equate:
		if t.To != nil {
			r.tab.use(t.To, t.To.Text, AsEquateSource, t.To.Position)
		}

	case *ast.Subroutine:
		for _, in := range t.Body {
			r.referenceStmt(in)
		}
	case *ast.LinkRoutine:
		for _, in := range t.Body {
			r.referenceStmt(in)
		}
	case *ast.ReturnFrom:
		r.name(t.Name, AsReturnFrom)
	case *ast.ExitFrom:
		r.name(t.Name, AsExitFrom)
	case *ast.LinkBack:

	case *ast.Call:
		r.name(t.Name, AsCallee)
		if t.Arg != nil {
			r.value(t.Arg.Value)
		}
		r.name(t.Exit, AsBranchTarget)
		r.checkCallAgreement(t)

	case *ast.If:
		if t.Cond != nil {
			for _, rel := range t.Cond.Rels {
				r.value(rel.X)
				r.value(rel.Y)
			}
		}
		for _, in := range t.Stmts() {
			r.referenceStmt(in)
		}

	case *ast.ChainFrom:
		r.value(t.Addr)
		r.name(t.Exit, AsBranchTarget)
		// CHANPT and CHLINK are read and written by the loop without being
		// named (lmap.txt 4.2.2). Registering the use is what keeps them from
		// being reported unused, and what reports them when a program uses
		// CHAIN FROM without declaring them.
		for _, v := range t.ImplicitVars() {
			r.tab.use(nil, v, AsValue, t.Position)
		}
		for _, in := range t.Body {
			r.referenceStmt(in)
		}

	case *ast.MoveFrom:
		r.value(t.From)
		r.value(t.To)
		r.value(t.Leng)
	case *ast.MStackFrom:
		r.value(t.From)
		r.value(t.Leng)
	case *ast.MUnstackFrom:
		r.value(t.From)
		r.value(t.To)
		r.value(t.Leng)

	case *ast.PRText:

	case *ast.Backspace:
		r.name(t.Var, AsValue)
		r.name(t.Giving, AsValue)
	case *ast.CharMatch:
		r.name(t.Ptr, AsValue)
		for _, arm := range t.Arms {
			r.value(arm.Char)
			r.name(arm.Target, AsBranchTarget)
		}
	case *ast.GoTo:
		r.name(t.Target, AsBranchTarget)
	case *ast.Scale:
		r.name(t.Var, AsValue)
		r.value(t.By)
		r.name(t.Giving, AsValue)
	case *ast.Set:
		for _, tg := range t.Targets {
			r.value(tg)
		}
		r.value(t.Value)
	case *ast.SetSW:
		for _, tg := range t.Targets {
			r.value(tg)
		}
		r.value(t.Value)
	case *ast.Stack:
		for _, v := range t.Values {
			r.value(v.Value)
		}
	case *ast.Unstack:
		for _, v := range t.Values {
			r.value(v.Value)
		}
	case *ast.Test:
		r.name(t.Var, AsValue)
		for _, l := range t.Targets {
			r.name(l, AsBranchTarget)
		}

	case *ast.DC:
		for _, e := range t.Args {
			r.value(e)
		}
	case *ast.OpMac:
		r.value(t.Dels)
		r.value(t.Marker)
		r.value(t.Number)
	}
}

func (r *resolver) name(i *ast.Ident, u Use) {
	if i == nil {
		return
	}
	r.tab.use(i, i.Text, u, i.Position)
}

// value registers every name inside an expression, giving AD, BLOCK and RL the
// use that says what their argument has to be.
func (r *resolver) value(e ast.Expr) {
	switch t := e.(type) {
	case nil:
		return
	case *ast.Ident:
		if t != nil {
			r.tab.use(t, t.Text, AsValue, t.Position)
		}
	case *ast.AD:
		r.name(t.Name, AsAddress)
	case *ast.BlockRef:
		r.name(t.Name, AsBlock)
	case *ast.RL:
		r.name(t.Name, AsRLTarget)
		r.value(t.Adjust)
	case *ast.OF:
		r.value(t.Arg)
	case *ast.Ind:
		r.value(t.Addr)
	case *ast.Binary:
		r.value(t.X)
		r.value(t.Y)
	case *ast.Unary:
		r.value(t.X)
	}
}

// checkCallAgreement compares a call against the declaration of what it calls
// (lmap.txt 4.1.3): an argument is supplied exactly when the subroutine
// declares a parameter, and EXIT exactly when it declares an exit.
//
// This is presence, not type, so it is inside what this pass promises.
func (r *resolver) checkCallAgreement(c *ast.Call) {
	if c.Name == nil {
		return
	}
	sym, ok := r.tab.Lookup(c.Name.Text)
	if !ok || !sym.IsDefined() {
		return // the undefined-name report covers it
	}
	switch sym.Kind {
	case SubName, LinkName, MDSub:
	default:
		return // the kind check covers it
	}
	switch {
	case c.Arg != nil && sym.Param == nil:
		r.errf(c.Arg.Position, "CALL %s takes no argument", sym.Name)
	case c.Arg == nil && sym.Param != nil:
		r.errf(c.Pos(), "CALL %s needs a %s argument", sym.Name, sym.Param.Name)
	}
	switch {
	case c.Exit != nil && !sym.HasExit:
		r.errf(c.Exit.Position, "CALL %s: the subroutine declares no EXIT", sym.Name)
	case c.Exit == nil && sym.HasExit:
		r.errf(c.Pos(), "CALL %s: the subroutine has an EXIT and the call does not name one", sym.Name)
	}
}

// --- pass 3: the report ----------------------------------------------------

// useWants says what kind a name must have to be used a given way.
var useWants = map[Use][]Kind{
	AsBranchTarget: {ProgLabel, MDLabel},
	AsCallee:       {SubName, LinkName, MDSub},
	AsAddress:      {DataLabel},
	AsBlock:        {BlockName},
	AsRLTarget:     {DataLabel},
	AsReturnFrom:   {SubName, LinkName},
	AsExitFrom:     {SubName},
	AsEquateSource: {Variable},
	AsValue:        {Variable, Constant},
}

func (r *resolver) report() {
	for _, sym := range r.tab.Symbols() {
		if !sym.IsDefined() {
			r.reportUndefined(sym)
			continue
		}
		for _, ref := range sym.Refs {
			want, ok := useWants[ref.Use]
			if !ok || hasKind(want, sym.Kind) {
				continue
			}
			// A SECTION or BLOCKDEC name is closed by its own name, and that
			// is the one place the name is not being used as a value.
			if ref.Use == AsEndName {
				continue
			}
			r.errf(ref.Pos, "%s is a %s and is used as %s", sym.Name, sym.Kind, ref.Use)
		}
		r.reportUnused(sym)
	}
}

func (r *resolver) reportUndefined(sym *Symbol) {
	if len(sym.Refs) == 0 {
		return
	}
	refs := append([]Reference(nil), sym.Refs...)
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].Pos.Before(refs[j].Pos) })
	first := refs[0]
	if len(refs) == 1 {
		r.errf(first.Pos, "%s is not declared, and is used as %s", sym.Name, first.Use)
		return
	}
	others := ""
	for _, ref := range refs[1:] {
		others += " " + ref.Pos.String()
	}
	r.errf(first.Pos, "%s is not declared, and is used as %s here and at%s",
		sym.Name, first.Use, others)
}

func (r *resolver) reportUnused(sym *Symbol) {
	if sym.Predefined || len(sym.Refs) > 0 {
		return
	}
	switch sym.Kind {
	case Variable:
		r.warnf(sym.Def, "%s is declared and never used", sym.Name)
	case ProgLabel:
		if _, entry := entryLabels[sym.Name]; entry {
			return // the host branches here (lmap.txt 7.3.1)
		}
		r.warnf(sym.Def, "nothing branches to %s", sym.Name)
	}
}

// sortedKeys gives a map a stable iteration order.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func hasKind(list []Kind, k Kind) bool {
	for _, want := range list {
		if want == k {
			return true
		}
	}
	return false
}
