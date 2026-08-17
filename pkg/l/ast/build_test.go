// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ast_test

import (
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/cst"
	"github.com/mdhender/maclo/pkg/l/scanner"
	"github.com/mdhender/maclo/pkg/l/token"
)

// build runs the three stages over a fragment. The fragment is wrapped in a
// SECTION so that the statements have somewhere legal to sit, and the
// resulting statement list is what a test looks at. sema is not run: these
// tests are about the shape of the tree.
func build(t *testing.T, body string) ([]ast.Stmt, token.Errors) {
	t.Helper()
	src := "PRGSTART\nSECTION MAIN,//A FRAGMENT//\n" + body + "\nENDSECT MAIN\nPRGEND\n"
	toks, errs := scanner.Scan([]byte(src))
	f, perrs := cst.Parse(toks)
	prog, berrs := ast.Build(f)
	errs.Merge(perrs)
	errs.Merge(berrs)
	for _, s := range prog.Stmts {
		if sec, ok := s.(*ast.Section); ok {
			return sec.Body, errs
		}
	}
	t.Fatalf("no SECTION in the tree")
	return nil, errs
}

func only(t *testing.T, body string) ast.Stmt {
	t.Helper()
	list, errs := build(t, body)
	if errs.HasErrors() {
		t.Fatalf("unexpected diagnostics:\n%v", errs)
	}
	if len(list) != 1 {
		t.Fatalf("got %d statements, want 1", len(list))
	}
	return list[0]
}

// TestIfHasOneTypeForBothForms. The manual makes something of the difference -
// it calls the newline of the one-line form a closing delimiter of two
// statements at once - but a consumer wants the condition and what it guards,
// and Stmts gives it either way.
func TestIfHasOneTypeForBothForms(t *testing.T) {
	oneLine := only(t, "        IF LEVEL = 0 THEN GO TO DONE").(*ast.If)
	if oneLine.Block {
		t.Error("the one-line form was built as a block")
	}
	if got := oneLine.Stmts(); len(got) != 1 {
		t.Fatalf("guards %d statements, want 1", len(got))
	} else if _, ok := got[0].(*ast.GoTo); !ok {
		t.Errorf("guards a %T, want a GO TO", got[0])
	}

	block := only(t, "        IF LEVEL = 0 THEN\n                SET LEVEL = 1\n                READ\n        END").(*ast.If)
	if !block.Block {
		t.Error("the block form was not built as a block")
	}
	if got := block.Stmts(); len(got) != 2 {
		t.Errorf("guards %d statements, want 2", len(got))
	}
}

// TestChainKeepsItsClosingLabel is the case that would have cost four defined
// labels in the L source of ML/I. A closer is folded onto its opener, and if
// the fold dropped the label the label would be gone.
func TestChainKeepsItsClosingLabel(t *testing.T) {
	chain := only(t, "        CHAIN FROM DELPT EXIT NOTFND\n                CALL CMPARE(CHANPT)PT EXIT BSC1\n[BSC1]  ENDCH").(*ast.ChainFrom)
	if chain.EndLabel == nil {
		t.Fatal("the label on the ENDCH was dropped")
	}
	if chain.EndLabel.Text != "BSC1" {
		t.Errorf("the closing label is %s, want BSC1", chain.EndLabel.Text)
	}
	if chain.Exit == nil || chain.Exit.Text != "NOTFND" {
		t.Errorf("the chain's exit is wrong")
	}
	if got := chain.ImplicitVars(); len(got) != 2 {
		t.Errorf("implicit variables %v, want CHANPT and CHLINK", got)
	}
}

// TestLayoutDoesNotChangeTheParse. L says layout is insignificant, so the two
// spellings of a stacked value have to produce the same tree. This is the
// reason nothing folds NAME( ) into one node before the statement's grammar
// says to.
func TestLayoutDoesNotChangeTheParse(t *testing.T) {
	spaced := only(t, "        STACK IDPT (PT) TEMP (NM) ON BSTACK").(*ast.Stack)
	tight := only(t, "        STACK IDPT(PT) TEMP(NM) ON BSTACK").(*ast.Stack)
	for i := range spaced.Values {
		a, b := spaced.Values[i], tight.Values[i]
		if a.Type != b.Type {
			t.Errorf("value %d: types differ: %s and %s", i, a.Type, b.Type)
		}
	}
	if len(spaced.Values) != 2 || len(tight.Values) != 2 {
		t.Errorf("got %d and %d values, want 2 each", len(spaced.Values), len(tight.Values))
	}
	if spaced.On != ast.BStack {
		t.Errorf("stacked on %s, want BSTACK", spaced.On)
	}
}

// TestMacrosBindOnlyToTheirOwnGroup. A word followed by a group is a macro
// call for six words and nothing else, which is what stops "PVNUM (NM)" from
// reading as a call of PVNUM.
func TestMacrosBindOnlyToTheirOwnGroup(t *testing.T) {
	set := only(t, "        SET IDLEN = IND(AD(KSPACE)PT)NM").(*ast.Set)
	ind, ok := set.Value.(*ast.Ind)
	if !ok {
		t.Fatalf("the value is a %T, want an IND", set.Value)
	}
	if ind.Type != ast.NM {
		t.Errorf("the indirect address is %s, want NM", ind.Type)
	}
	ad, ok := ind.Addr.(*ast.AD)
	if !ok {
		t.Fatalf("the address is a %T, want an AD", ind.Addr)
	}
	if ad.Name.Text != "KSPACE" {
		t.Errorf("AD names %s, want KSPACE", ad.Name.Text)
	}

	// The same shape without a macro name is a value and a separate group.
	stack := only(t, "        STACK PVNUM (NM) ON FSTACK").(*ast.Stack)
	if _, ok := stack.Values[0].Value.(*ast.Ident); !ok {
		t.Errorf("the stacked value is a %T, want a plain name", stack.Values[0].Value)
	}
}

// TestSubroutineHasAtMostOneExit. L's grammar is singular, and LOWL's
// multi-exit jump table is not a thing that belongs in this tree.
func TestSubroutineHasAtMostOneExit(t *testing.T) {
	src := "PRGSTART\nSECTION MAINSUBS,//SUBS//\n" +
		"SUBROUTINE ADVNCE EXIT //END OF PIECE OF TEXT//\n        READ\nENDSUB\n" +
		"SUBROUTINE CKVALY(PARPT)\n        READ\nENDSUB\n" +
		"ENDSECT MAINSUBS\nPRGEND\n"
	toks, _ := scanner.Scan([]byte(src))
	f, _ := cst.Parse(toks)
	prog, errs := ast.Build(f)
	if errs.HasErrors() {
		t.Fatalf("unexpected diagnostics:\n%v", errs)
	}
	body := prog.Stmts[1].(*ast.Section).Body

	withExit := body[0].(*ast.Subroutine)
	if !withExit.HasExit {
		t.Error("the EXIT was not recorded")
	}
	if withExit.ExitComment != "END OF PIECE OF TEXT" {
		t.Errorf("the exit comment is %q", withExit.ExitComment)
	}
	if withExit.Param != nil {
		t.Errorf("a parameter was invented: %v", withExit.Param)
	}

	withParam := body[1].(*ast.Subroutine)
	if withParam.HasExit {
		t.Error("an EXIT was invented")
	}
	if withParam.Param == nil || withParam.Param.Type != ast.PT {
		t.Errorf("the parameter is %v, want PARPT", withParam.Param)
	}
}

// TestLinkRoutineClosesWithENDSUB is why the L source of ML/I holds one more
// ENDSUB than SUBROUTINE.
func TestLinkRoutineClosesWithENDSUB(t *testing.T) {
	src := "PRGSTART\nSECTION MAINSUBS,//SUBS//\nLINKROUTINE LINKED\n        READ\nENDSUB\nENDSECT MAINSUBS\nPRGEND\n"
	toks, _ := scanner.Scan([]byte(src))
	f, _ := cst.Parse(toks)
	prog, errs := ast.Build(f)
	if errs.HasErrors() {
		t.Fatalf("unexpected diagnostics:\n%v", errs)
	}
	body := prog.Stmts[1].(*ast.Section).Body
	if _, ok := body[0].(*ast.LinkRoutine); !ok {
		t.Fatalf("got a %T, want a LINKROUTINE", body[0])
	}
}

// TestPrefixAfterTHENBelongsToTheInnerStatement. Two lines of the L source of
// ML/I write it, and the manual documents neither the position nor the case.
func TestPrefixAfterTHENBelongsToTheInnerStatement(t *testing.T) {
	s := only(t, "[GXP5]  IF NEGVAL = 1 THEN /-OVP-/SET MEVAL = -MEVAL").(*ast.If)
	if s.HasPrefix("OVP") {
		t.Error("the prefix was attached to the IF")
	}
	if s.Label == nil || s.Label.Text != "GXP5" {
		t.Error("the label was lost")
	}
	inner := s.Then
	if inner == nil {
		t.Fatal("the guarded statement is missing")
	}
	if !inner.Common().HasPrefix("OVP") {
		t.Errorf("the SET did not get the prefix: %v", inner.Common().Prefixes)
	}
}

func TestBuildErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"unclosed section", "        READ", ""}, // the wrapper closes it
		{"a closer that does not match what is open", "        END", "END closes SECTION"},
		{"a block IF cannot follow THEN", "        IF LEVEL = 0 THEN IF DEPTH = 0 THEN READ", "cannot follow THEN"},
		{"a chain cannot follow THEN", "        IF LEVEL = 0 THEN CHAIN FROM DELPT EXIT DONE", "cannot follow THEN"},
		{"IND needs a type", "        SET A = IND(SPT)", "expected the type"},
		{"CALL argument needs a type", "        CALL SUBR(TOTAL)", "expected the type"},
		{"a comparison is required", "        IF LEVEL THEN READ", "expected =, NE, GR, GE or LE"},
		{"mixed joins", "        IF A = 0 & B = 0 | C = 0 THEN READ", "all & or all |"},
		{"leftover arguments", "        READ SOMETHING", "unexpected the word SOMETHING"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := build(t, tc.body)
			joined := errs.Error()
			if tc.want == "" {
				if errs.HasErrors() {
					t.Errorf("unexpected diagnostics:\n%s", joined)
				}
				return
			}
			if !strings.Contains(joined, tc.want) {
				t.Errorf("diagnostics are\n%s\nwant one mentioning %q", joined, tc.want)
			}
		})
	}
}
