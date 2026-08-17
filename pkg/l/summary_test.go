// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package l_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/l"
	"github.com/mdhender/maclo/pkg/l/sema"
	"github.com/mdhender/maclo/pkg/l/stmt"
)

// summarySource is small enough to count by hand, which is the only way a
// counter can be checked. It holds one of each of the things the summary has
// an opinion about: a block, a subroutine, a one-line IF and a block IF, a
// chain, and a name it never declares.
const summarySource = `PRGSTART

SECTION VARS,//THREE VARIABLES AND A BLOCK//
BLOCKDEC SDB,//ONE BLOCK//
        DEC SPT INIT ZEROPT
ENDBLOCK SDB
        DEC CHANPT INIT NULLPT
        DEC CHLINK INIT 0
        DEC DELPT INIT NULLPT
        DEC LEVEL INIT 0
ENDSECT VARS

SECTION MAIN,//THE STATEMENTS//
[BEGIN] IF LEVEL = 0 THEN GO TO DONE
        IF LEVEL = 1 THEN
                SET LEVEL = 0
                GO TO NOWHER
        END
        CHAIN FROM DELPT EXIT DONE
        ENDCH
[DONE]  GO TO BEGIN
ENDSECT MAIN

SECTION MAINSUBS,//ONE SUBROUTINE//
SUBROUTINE ADVNCE
        READ
        RETURN FROM ADVNCE
ENDSUB
ENDSECT MAINSUBS

PRGEND
`

func TestSummaryCounts(t *testing.T) {
	s := l.Parse([]byte(summarySource)).Summary()

	if s.Lines != 31 {
		t.Errorf("got %d lines, want 31", s.Lines)
	}
	// PRGSTART, 3 SECTION, BLOCKDEC, 5 DEC, 2 IF, 3 GO TO, SET,
	// CHAIN FROM, SUBROUTINE, READ, RETURN FROM, PRGEND. The GO TO after THEN
	// is one of the three: a one-line IF holds a statement like any other, and
	// that is why the corpus counts 255 GO TO on 122 lines that start with one.
	if s.Statements != 21 {
		t.Errorf("got %d statements, want 21", s.Statements)
	}
	for kind, want := range map[stmt.Kind]int{
		stmt.If: 2, stmt.GoTo: 3, stmt.Dec: 5, stmt.BlockDec: 1,
		stmt.ChainFrom: 1, stmt.Subroutine: 1, stmt.Section: 3,
	} {
		if s.ByStatement[kind] != want {
			t.Errorf("%s: got %d, want %d", kind, s.ByStatement[kind], want)
		}
	}
	if got := strings.Join(s.Sections, " "); got != "VARS MAIN MAINSUBS" {
		t.Errorf("sections are %q", got)
	}

	// The one-line IF opens nothing, so only the block IF has a depth, and the
	// chain is not inside it.
	if s.MaxIfDepth != 1 || s.MaxChainDepth != 1 {
		t.Errorf("depths are IF %d and CHAIN %d, want 1 and 1", s.MaxIfDepth, s.MaxChainDepth)
	}

	if s.Undeclared != 1 {
		t.Errorf("got %d undeclared names, want 1 (NOWHER)", s.Undeclared)
	}
	if s.ByName[sema.Variable] != 5 {
		t.Errorf("got %d variables, want 5", s.ByName[sema.Variable])
	}
	// ZEROPT and NULLPT, and SDBSZ is derived rather than predefined.
	if s.Predefined != 2 {
		t.Errorf("got %d of L's own names referred to, want 2", s.Predefined)
	}
	if s.Errors != 1 {
		t.Errorf("got %d errors, want 1", s.Errors)
	}
}

// TestSummaryOfABrokenSourceStillCounts. Every stage accumulates rather than
// stopping, so a summary of a file that does not resolve is still a summary of
// what is in it - which is what makes the number useful while the file is
// being fixed.
func TestSummaryOfABrokenSourceStillCounts(t *testing.T) {
	broken := strings.Replace(summarySource, "ENDSECT MAIN", "ENDSECT WRONG", 1)
	s := l.Parse([]byte(broken)).Summary()
	if s.Errors == 0 {
		t.Fatal("the source was accepted")
	}
	if s.Statements == 0 {
		t.Error("nothing was counted")
	}
}

// TestWriteSummaryIsStable. The tallies come out of maps. Ranging over one
// directly would give a different report on every run, and this package has
// already been caught by exactly that once.
func TestWriteSummaryIsStable(t *testing.T) {
	s := l.Parse([]byte(summarySource)).Summary()
	var first bytes.Buffer
	if err := l.WriteSummary(&first, s); err != nil {
		t.Fatal(err)
	}
	for i := range 20 {
		var again bytes.Buffer
		if err := l.WriteSummary(&again, l.Parse([]byte(summarySource)).Summary()); err != nil {
			t.Fatal(err)
		}
		if again.String() != first.String() {
			t.Fatalf("run %d differs:\n%s\nwant:\n%s", i, again.String(), first.String())
		}
	}
}
