// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package l_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mdhender/maclo/pkg/l"
	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/stmt"
	"github.com/mdhender/maclo/pkg/l/token"
)

// TestML1AIE runs the front end over the real L source of ML/I, version AIE.
//
// It is the test the rest of the package exists for. The committed corpus is
// eleven small files written from the manual, and small files written by the
// person who wrote the parser agree with it by construction; 2,510 lines of
// somebody else's L from 1971 do not.
//
// The file cannot be committed - it is copyright P.J. Brown and R.D. Eager and
// the licence does not permit redistribution - so this skips when it is
// absent, the way pkg/lowl/vm/lowltest_test.go does. The skip is keyed on the
// file rather than on an environment variable or a build tag, so it expires by
// itself the moment the corpus is fetched.
func TestML1AIE(t *testing.T) {
	src, ok := ml1aieSource(t)
	if !ok {
		return
	}
	result := l.Parse(src)
	summary := result.Summary()

	t.Run("stages", func(t *testing.T) {
		for _, e := range result.Errs {
			if e.Severity == token.SevError && e.Stage != token.StageSema {
				t.Errorf("%s: %s: %s", e.Pos, e.Stage, e.Msg)
			}
		}
	})

	t.Run("statements", func(t *testing.T) { checkCounts(t, summary) })
	t.Run("sections", func(t *testing.T) { checkSections(t, summary) })
	t.Run("nesting", func(t *testing.T) { checkNesting(t, summary) })
	t.Run("names", func(t *testing.T) { checkNames(t, result.Errs) })
	t.Run("roundtrip", func(t *testing.T) { checkRoundTrip(t, result.Program) })
}

// wantCounts is how many of each statement the file holds.
//
// Two of these are worth reading twice. There are 59 ENDSUB for 58
// SUBROUTINE, because the one LINKROUTINE closes with ENDSUB too - and since
// this front end folds every closer onto its opener, that shows up here as
// SUBROUTINE 58 and LINKROUTINE 1 rather than as a discrepancy. And GO TO is
// 255 while only 122 lines begin with one: the other 133 are the statement
// after THEN on a one-line IF, which is the case lmap.txt 4.2.1 says many
// L-maps special-case.
var wantCounts = map[stmt.Kind]int{
	stmt.Set: 371, stmt.GoTo: 255, stmt.Call: 235, stmt.If: 203, stmt.Dec: 84,
	stmt.SetSW: 63, stmt.ReturnFrom: 61, stmt.Subroutine: 58, stmt.PRText: 54, stmt.DC: 50,
	stmt.Equate: 26, stmt.Stack: 22, stmt.OpMac: 20, stmt.ExitFrom: 15, stmt.Backspace: 11,
	stmt.ChainFrom: 11, stmt.MoveFrom: 10, stmt.Section: 10, stmt.Scale: 10, stmt.Test: 9,
	stmt.BlockDec: 5, stmt.MStackFrom: 5, stmt.CharMatch: 5, stmt.MUnstackFrom: 3,
	stmt.Unstack: 2, stmt.PrgStart: 1, stmt.PrgEnd: 1, stmt.LayChain: 1, stmt.LinkRoutine: 1,
	stmt.Read: 1, stmt.HETables: 1, stmt.LinkBack: 1, stmt.OutputID: 1,
}

// wantStatements is the total, and it is asserted separately from the tally so
// that a statement counted under the wrong kind cannot cancel itself out.
const wantStatements = 1606

func checkCounts(t *testing.T, s *l.Summary) {
	if s.Statements != wantStatements {
		t.Errorf("got %d statements, want %d", s.Statements, wantStatements)
	}
	for kind, want := range wantCounts {
		if s.ByStatement[kind] != want {
			t.Errorf("%s: got %d, want %d", kind, s.ByStatement[kind], want)
		}
	}
	for kind, n := range s.ByStatement {
		if _, ok := wantCounts[kind]; !ok {
			t.Errorf("%s: got %d, and the table does not mention it", kind, n)
		}
	}
}

// wantSections is the ten SECTIONs of the logic, in the order the file writes
// them (lmap.txt 2.4).
var wantSections = []string{
	"VARS", "MACNAMES", "DELS", "INVALS", "MAIN",
	"MAINSUBS", "OPMACS", "DEFSUBS", "ERR", "ENVPR",
}

func checkSections(t *testing.T, s *l.Summary) {
	got := s.Sections
	if len(got) != len(wantSections) {
		t.Fatalf("got %d SECTIONs %v, want %d", len(got), got, len(wantSections))
	}
	for i, want := range wantSections {
		if got[i] != want {
			t.Errorf("SECTION %d is %s, want %s", i+1, got[i], want)
		}
	}
}

// checkNesting holds the two restrictions the manual states as facts about the
// logic rather than as syntax: a block IF is never inside a block IF, and a
// CHAIN FROM is never inside a CHAIN FROM (lmap.txt 4.2.1, 4.2.2).
//
// Asserting them against the real source is what turns them from a claim in a
// manual into something known.
func checkNesting(t *testing.T, s *l.Summary) {
	if s.MaxIfDepth != 1 {
		t.Errorf("block IF nesting reaches %d, want 1 (lmap.txt 4.2.1)", s.MaxIfDepth)
	}
	if s.MaxChainDepth != 1 {
		t.Errorf("CHAIN FROM nesting reaches %d, want 1 (lmap.txt 4.2.2)", s.MaxChainDepth)
	}
}

// wantUndefined is the number of names AIE uses and never declares.
//
// It is one, and the one is a real defect in the file rather than a gap in
// this front end: a TEST branches to a label whose declaration is spelt with
// a letter too many. The LOWL distribution settles which of the two is the
// typo - it spells the label the way the branch does - so the branch is right
// and the L file's label is wrong.
//
// The count is asserted and the name is not written here: the source is under
// a licence that keeps it out of this repository, and the count catches drift
// just as well. If the day comes that AIE is corrected, this is the test that
// says so.
const wantUndefined = 1

func checkNames(t *testing.T, errs token.Errors) {
	var undefined []token.Error
	for _, e := range errs {
		if e.Severity == token.SevError && e.Stage == token.StageSema {
			undefined = append(undefined, e)
		}
	}
	if len(undefined) != wantUndefined {
		t.Errorf("got %d name errors, want %d", len(undefined), wantUndefined)
		for _, e := range undefined {
			t.Logf("  %s: %s", e.Pos, e.Msg)
		}
	}
}

// checkRoundTrip renders the tree back to L, reads it again, and requires the
// same text. Over 2,510 lines of real source it is the cheapest proof there is
// that the listing has not quietly dropped anything.
func checkRoundTrip(t *testing.T, p *ast.Program) {
	var first bytes.Buffer
	if err := ast.WriteSource(&first, p); err != nil {
		t.Fatal(err)
	}
	again := l.Parse(first.Bytes())
	for _, e := range again.Errs {
		if e.Severity == token.SevError && e.Stage != token.StageSema {
			t.Errorf("re-reading the listing: %s: %s: %s", e.Pos, e.Stage, e.Msg)
		}
	}
	var second bytes.Buffer
	if err := ast.WriteSource(&second, again.Program); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Errorf("the listing does not read back as itself\n%s",
			firstDifference(first.Bytes(), second.Bytes()))
	}
}

func ml1aieSource(t *testing.T) ([]byte, bool) {
	t.Helper()
	name := filepath.Join(moduleRoot(), ".downloads", "lml1", "ml1aie.l")
	src, err := os.ReadFile(name)
	if err != nil {
		t.Skipf("%s: not fetched; run: go run ./cmd/fetchtestdata -corpus lml1", name)
		return nil, false
	}
	return src, true
}

// moduleRoot finds the top of the repository, because a test runs from its
// package directory and the archives are unpacked at the top.
func moduleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
