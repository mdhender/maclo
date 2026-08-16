// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/ml1"
)

// TestStorageExhaustionIsReported covers what happens when a process runs out
// of workspace.
//
// This is not a golden file, and cannot be: what reaches the debugging stream
// is the MI-logic's own diagnostic, in upstream's wording, so the corpus rules
// keep it out of a committed file. What is checked here instead is that the
// diagnostic happens at all, which is the part that was wrong.
//
// It was wrong because the machine used to stop. The kernel manual maps the
// stacking statements as ending in "GOGE ERLSO", a branch to a label every
// MI-logic has, and treating that as a fault the machine reports itself threw
// away everything the program would have said: the message, the count in S5,
// and the context print-out that says where the storage went. A process would
// abort with an empty debugging stream, which reads like a process that
// finished cleanly.
func TestStorageExhaustionIsReported(t *testing.T) {
	// a macro that recurses far enough to exhaust a small workspace. The
	// depth reached is implementation dependent, which is why the upstream
	// overflow case has its debugging stream excluded, so this asks only that
	// the storage ran out and not where.
	const source = `MCSKIP MT,<>
MCINS %.
MCDEF DOWN; AS <MCGO L1 IF %A1. GR 0
MCGO L0
%L1.DOWN %%A1.-1.;>
DOWN 500;
`
	var out, dbg bytes.Buffer
	res, err := ml1.Run(ml1.Job{
		Inputs:     []ml1.Input{ml1.StringInput("deep.ml1", source)},
		Outputs:    []io.Writer{&out},
		Debug:      &dbg,
		Workspace:  600, // small, so this finishes quickly
		LOWLSource: lowlSource(),
	})
	if errors.Is(err, ml1.ErrNoEngineSource) {
		t.Skipf("%v; run: go run ./cmd/fetchtestdata\n", err)
	}

	// The process reports an error rather than dying: ERLSO counts the error
	// in S5 and then ends through the ordinary finalisation, so this is a
	// process that finished and reported, which appendix AA gives the exit
	// status 254.
	if !errors.Is(err, ml1.ErrProcessErrors) {
		t.Errorf("run: want ErrProcessErrors: got %v", err)
	}
	if res.Errors == 0 {
		t.Errorf("S5: want the error counted: got %d", res.Errors)
	}
	if res.Fatal {
		t.Errorf("fatal: want false; the process ended through its own finalisation")
	}
	if got, want := res.ExitStatus(), 254; got != want {
		t.Errorf("exit status: want %d: got %d", want, got)
	}

	// The wording belongs to upstream, so this checks that something was said
	// and that it was said in the right shape, rather than quoting it: an
	// error print-out followed by the context that leads back to the source.
	if dbg.Len() == 0 {
		t.Fatalf("debugging stream: want a diagnostic: got nothing")
	}
	for _, want := range []string{"Error(s)", "source text"} {
		if !strings.Contains(dbg.String(), want) {
			t.Errorf("debugging stream: want it to mention %q, got:\n%s", want, dbg.String())
		}
	}
}
