// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package cst

import (
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/l/scanner"
	"github.com/mdhender/maclo/pkg/l/stmt"
)

func parse(t *testing.T, src string) (*File, string) {
	t.Helper()
	toks, errs := scanner.Scan([]byte(src))
	if len(errs) != 0 {
		t.Fatalf("scanner diagnostics:\n%v", errs)
	}
	f, perrs := Parse(toks)
	var msgs []string
	for _, e := range perrs {
		msgs = append(msgs, e.Msg)
	}
	return f, strings.Join(msgs, "; ")
}

func TestParseLines(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		head      stmt.Kind
		want      string
	}{
		{"assignment", "SET SPT = IDPT-OF(LCH)", stmt.Set, "SET SPT = IDPT-OF(LCH)"},
		{"two word head", "GO TO MBEGIN", stmt.GoTo, "GO TO MBEGIN"},
		{"labelled", "[BSC1]  ENDCH", stmt.EndCh, "[BSC1] ENDCH"},
		{"label then prefix", "[BEGIN] /-IN-/SET A = NULLPT", stmt.Set, "[BEGIN] /-IN-/ SET A = NULLPT"},
		{"prefix then label", "/-CSS-/[FNCTEX] CALL OPEXIT", stmt.Call, "[FNCTEX] /-CSS-/ CALL OPEXIT"},
		{"section with a comment argument", "SECTION VARS,//DECLARATIONS//", stmt.Section, "SECTION VARS, //DECLARATIONS//"},
		{"subroutine with an exit comment", "SUBROUTINE ADVNCE EXIT //END OF TEXT//", stmt.Subroutine, "SUBROUTINE ADVNCE EXIT //END OF TEXT//"},
		// the trailing type suffix comes back spaced: at this stage PT is just
		// the next word, and nothing has decided it belongs to the group
		{"nested groups", "CALL MDTEST(FFPT-OF(LNM+LCH))PT EXIT GTLOOP", stmt.Call, "CALL MDTEST(FFPT-OF(LNM+LCH)) PT EXIT GTLOOP"},
		{"indirect", "IF IND(IDPT)CH = STOPCODE THEN GO TO MNSTOP", stmt.If, "IF IND(IDPT) CH = STOPCODE THEN GO TO MNSTOP"},
		{"prtext", "[PRT4]  PRTEXT[SKIP]", stmt.PRText, "[PRT4] PRTEXT[SKIP]"},
		{"stack with tight type tags", "STACK PARSW(SW) 3(NM) ON FSTACK", stmt.Stack, "STACK PARSW(SW) 3(NM) ON FSTACK"},
		{"stack with spaced type tags", "STACK IDPT (PT) ON BSTACK", stmt.Stack, "STACK IDPT(PT) ON BSTACK"},
		{"test with a label list", "TEST BESTPL GOING A1,B2,C3", stmt.Test, "TEST BESTPL GOING A1, B2, C3"},
		{"no arguments", "ENDSUB", stmt.EndSub, "ENDSUB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, errs := parse(t, tc.src)
			if errs != "" {
				t.Fatalf("unexpected diagnostics: %s", errs)
			}
			if len(f.Lines) != 1 {
				t.Fatalf("got %d lines, want 1", len(f.Lines))
			}
			line := f.Lines[0]
			if line.Head != tc.head {
				t.Errorf("head is %s, want %s", line.Head, tc.head)
			}
			if got := line.String(); got != tc.want {
				t.Errorf("\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// TestTriviaAttachesForwards checks that a heading or a standalone comment
// becomes the lead of the statement below it rather than a line of its own.
func TestTriviaAttachesForwards(t *testing.T) {
	src := "/+MAIN SCANNING ROUTINE+/\n//AND A NOTE//\n\n[MBEGIN]        SET FFPT = PVARPT\n"
	f, errs := parse(t, src)
	if errs != "" {
		t.Fatalf("unexpected diagnostics: %s", errs)
	}
	if len(f.Lines) != 1 {
		t.Fatalf("got %d lines, want 1: %v", len(f.Lines), f.Lines)
	}
	if got := len(f.Lines[0].Lead); got != 2 {
		t.Fatalf("got %d lead comments, want 2", got)
	}
	if got := f.Lines[0].Lead[0].Text; got != "MAIN SCANNING ROUTINE" {
		t.Errorf("first lead is %q", got)
	}
}

// TestTrailingTriviaIsKept: the L source of ML/I writes a closing comment
// between its last ENDSECT and PRGEND, and a file could end with one.
func TestTrailingTriviaIsKept(t *testing.T) {
	f, errs := parse(t, "PRGEND\n//END OF LOGIC//\n")
	if errs != "" {
		t.Fatalf("unexpected diagnostics: %s", errs)
	}
	if len(f.Lines) != 1 || len(f.Tail) != 1 {
		t.Fatalf("got %d lines and %d tail, want 1 and 1", len(f.Lines), len(f.Tail))
	}
}

func TestParseErrors(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
	}{
		{"unknown statement", "SETT A = 0", `"SETT" is not a statement of L`},
		{"a lone GO", "GO MBEGIN", `"GO" is not a statement of L`},
		{"keyword as a head", "THEN GO TO X", `"THEN" is not a statement of L`},
		{"label with no statement", "[ORPHAN]", "has no statement on its line"},
		{"prefix with no statement", "/-OVP-/", "statement prefix with no statement"},
		{"two labels", "[A1][B2] READ", "second label"},
		{"statement starts with punctuation", "= A", "expected a statement"},
		{"unclosed group", "SET A = OF(LCH", "unclosed ("},
		{"stray close paren", "SET A = B)", "no ( opened here"},
		{"label mid statement", "SET A = [B1]", "in the middle of a statement"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := parse(t, tc.src)
			if !strings.Contains(errs, tc.want) {
				t.Errorf("diagnostics are %q, want one mentioning %q", errs, tc.want)
			}
		})
	}
}

// TestRecoveryIsPerStatement is why the cst carries errors instead of
// returning one. A bad line must cost one diagnostic, not swallow the rest of
// the file.
func TestRecoveryIsPerStatement(t *testing.T) {
	src := "SET A = 0\nSETT B = 1\nSET C = 2\nWHAT D\nSET E = 4\n"
	f, _ := parse(t, src)
	if len(f.Lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(f.Lines))
	}
	for i, want := range []stmt.Kind{stmt.Set, stmt.Unknown, stmt.Set, stmt.Unknown, stmt.Set} {
		if got := f.Lines[i].Head; got != want {
			t.Errorf("line %d is %s, want %s", i+1, got, want)
		}
	}
	for _, i := range []int{1, 3} {
		if len(f.Lines[i].Errs) == 0 {
			t.Errorf("line %d carries no error", i+1)
		}
	}
	for _, i := range []int{0, 2, 4} {
		if len(f.Lines[i].Errs) != 0 {
			t.Errorf("line %d carries %v", i+1, f.Lines[i].Errs)
		}
	}
}

// TestPrefixAfterTHEN is a case neither the manual nor the design anticipated,
// and the L source of ML/I has two of them: a one-line IF whose guarded
// statement carries a prefix, "IF NEGVAL = 1 THEN /-OVP-/SET MEVAL = -MEVAL".
// The prefix belongs to the SET, so the cst records it as an argument and
// ast/build.go attaches it to the inner statement.
func TestPrefixAfterTHEN(t *testing.T) {
	f, errs := parse(t, "[GXP5]  IF NEGVAL = 1 THEN /-OVP-/SET MEVAL = -MEVAL")
	if errs != "" {
		t.Fatalf("unexpected diagnostics: %s", errs)
	}
	line := f.Lines[0]
	if line.Head != stmt.If {
		t.Fatalf("head is %s, want IF", line.Head)
	}
	if len(line.Prefixes) != 0 {
		t.Errorf("the prefix was attached to the IF; it belongs to the SET")
	}
	var found bool
	for _, a := range line.Args {
		if a.Kind == ArgPrefix && a.Text == "OVP" {
			found = true
		}
	}
	if !found {
		t.Errorf("no OVP prefix among the arguments: %s", ArgsText(line.Args))
	}
}

func TestHasPrefix(t *testing.T) {
	f, _ := parse(t, "/-IN-/SET A = 0")
	line := f.Lines[0]
	if !line.HasPrefix("IN") {
		t.Error("IN prefix not found")
	}
	if line.HasPrefix("OVP") {
		t.Error("OVP prefix found where there is none")
	}
}
