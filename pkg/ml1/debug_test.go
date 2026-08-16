// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/ml1"
)

// The debugging stream, which the golden corpus cannot reach.
//
// Every case in testdata/local relies on the harness reading a missing .err as
// "this stream must be empty", so the whole diagnostic path — error messages,
// context print-outs, MCNOTE, warning markers, the end of process report — is
// asserted here instead. The corpus rules keep it out of a golden file because
// the wording of a system message is upstream's expression and a golden would
// be a machine readable copy of it; pkg/ml1/storage_test.go is the existing
// example of the pattern these tests extend.
//
// Two kinds of assertion appear below, and which one a case gets is not a
// matter of taste:
//
//   - Where every byte on the stream came from the source in the test, the
//     comparison is exact. MCSET S4=1 makes MCNOTE write its argument and
//     nothing else, so that case is byte for byte against text this repository
//     wrote. It is the only slice of this stream where that is possible.
//   - Everywhere else the assertion is on substrings, chosen so that the test
//     says what shape the diagnostic has without quoting more of it than a
//     reader needs to see what is being checked.
//
// The substrings are also chosen to avoid the two places the 1986 LOWL source
// and the CKQ golden files disagree, both described in BURNDOWN.md: a message
// that ends in a run of newlines, and the spacing around "with arguments" and
// the enumerated arguments under it. Neither is a defect on this side, and
// pinning either would pin the skew.
//
// Every case here was run against the reference implementation as well, so that
// what is asserted is behaviour and not just what this engine happens to do.

// runDebug runs one source through the engine in process and hands back both
// streams. It skips rather than fails when the LOWL source of ML/I is not on
// this machine, the same way the golden corpus does, keyed on the sentinel so
// that the skip expires by itself.
//
// DebugWidth is NeverWrap because a hard wrap at column 72 is the upstream
// harness's device and would break a substring in half at an arbitrary place.
func runDebug(t *testing.T, source string) (out, dbg string, res ml1.Result, err error) {
	t.Helper()
	var o, d bytes.Buffer
	res, err = ml1.Run(ml1.Job{
		Inputs:     []ml1.Input{ml1.StringInput("debug.ml1", source)},
		Outputs:    []io.Writer{&o},
		Debug:      &d,
		Workspace:  ml1.DefaultWorkspace,
		DebugWidth: ml1.NeverWrap,
		LOWLSource: lowlSource(),
	})
	if errors.Is(err, ml1.ErrNoEngineSource) {
		t.Skipf("%v; run: go run ./cmd/fetchtestdata\n", err)
	}
	return o.String(), d.String(), res, err
}

// wantMentions checks that each of want appears on the stream, reporting the
// whole stream once if any is missing so that a failure can be read.
func wantMentions(t *testing.T, what, stream string, want ...string) {
	t.Helper()
	missing := false
	for _, s := range want {
		if !strings.Contains(stream, s) {
			t.Errorf("%s: want it to mention %q\n", what, s)
			missing = true
		}
	}
	if missing {
		t.Errorf("%s was:\n%s\n", what, stream)
	}
}

// TestDebugNoteWithoutContext is the one byte-exact assertion this stream
// allows.
//
// S4 is the option on MCNOTE: set it to one and the context print-out is
// suppressed, so all that reaches the debugging stream is the argument,
// preceded and followed by a newline. Nothing of the processor's own wording is
// left, which is what makes an exact comparison legitimate here.
//
// The output stream is empty because an operation macro consumes the newline
// that delimits it, so three lines of MCSET and MCNOTE produce no value text at
// all. That is worth pinning: it is the difference between a macro being obeyed
// and a macro being copied.
func TestDebugNoteWithoutContext(t *testing.T) {
	const source = `MCSET S4=1
MCNOTE Counting reached 7.
MCNOTE and then stopped.
`
	out, dbg, res, err := runDebug(t, source)
	if err != nil {
		t.Fatalf("run: %v\n", err)
	}
	if want := "\nCounting reached 7.\n\nand then stopped.\n"; dbg != want {
		t.Errorf("debugging stream:\n\twant %q\n\t got %q\n", want, dbg)
	}
	if out != "" {
		t.Errorf("output stream: want it empty: got %q\n", out)
	}
	// MCNOTE is the user's own message, not an error, so S5 does not move and
	// the process is a success.
	if res.Errors != 0 {
		t.Errorf("S5: want 0, MCNOTE is not an error: got %d\n", res.Errors)
	}
	if got, want := res.ExitStatus(), 0; got != want {
		t.Errorf("exit status: want %d: got %d\n", want, got)
	}
}

// TestDebugNoteWithContext covers the other half of S4: with it left at zero,
// MCNOTE's argument is followed by a print-out of the context of the call.
//
// The context is the part that makes the debugging stream worth having. It
// names the text the note was written in, the line reached within it, and then
// walks outwards through every call and insert still being processed until it
// reaches the source. This case has two levels, so both "of macro" and "of
// source text" have to appear.
func TestDebugNoteWithContext(t *testing.T) {
	const source = `MCSKIP MT,<>
MCINS %.
MCDEF CHECK; AS <MCNOTE looked at %A1.
seen %A1.>
CHECK the first;
`
	out, dbg, res, err := runDebug(t, source)
	if err != nil {
		t.Fatalf("run: %v\n", err)
	}
	if want := "seen the first\n"; out != want {
		t.Errorf("output stream:\n\twant %q\n\t got %q\n", want, out)
	}
	wantMentions(t, "debugging stream", dbg,
		// the note itself, evaluated: %A1. was inserted before it was written
		"looked at the first",
		// the context, from the inside out. "line 2 of macro CHECK" is the
		// second line of the replacement text, not of the source.
		"detected in",
		"line 2 of macro CHECK",
		"called from",
		"line 5 of source text",
	)
	if res.Errors != 0 {
		t.Errorf("S5: want 0, MCNOTE is not an error: got %d\n", res.Errors)
	}
}

// TestDebugWarningMarker covers MCWARN, and the S3 option that goes with it.
//
// A warning marker is a character that may introduce a macro call. One that is
// not followed by a macro name is copied over to the value text as if it had
// never been a construction name — but by default it is also reported, because
// the usual reason for defining a marker is that every occurrence of it is
// meant to be a call.
//
// S3 is the switch for source text where that is not so, and it suppresses the
// message only: what reaches the output is the same either way. Both halves are
// run here, because a suppression that also changed the value text would be a
// different feature.
func TestDebugWarningMarker(t *testing.T) {
	const (
		body = `MCSKIP MT,<>
MCDEF PIG AS <POG>
MCWARN +
+PIG then +NOTMAC, and a bare + at the end.
`
		suppressed = `MCSKIP MT,<>
MCDEF PIG AS <POG>
MCSET S3=1
MCWARN +
+PIG then +NOTMAC, and a bare + at the end.
`
		// +PIG is a call and is replaced. The other two markers are followed by
		// something that is not a macro name, so each is copied through
		// unchanged along with the atom after it.
		value = "POG then +NOTMAC, and a bare + at the end.\n"
	)

	out, dbg, res, err := runDebug(t, body)
	if !errors.Is(err, ml1.ErrProcessErrors) {
		t.Errorf("run: want ErrProcessErrors: got %v\n", err)
	}
	if out != value {
		t.Errorf("output stream:\n\twant %q\n\t got %q\n", value, out)
	}
	// NOTMAC is a name that is not defined; "at" is the atom after the bare
	// marker. Both are reported, and each report is counted in S5.
	wantMentions(t, "debugging stream", dbg,
		`after warning, viz "NOTMAC"`,
		`after warning, viz "at"`,
		"line 4 of source text",
	)
	if got, want := res.Errors, 2; got != want {
		t.Errorf("S5: want %d, one per marker that named nothing: got %d\n", want, got)
	}
	if got, want := res.ExitStatus(), 254; got != want {
		t.Errorf("exit status: want %d: got %d\n", want, got)
	}

	out, dbg, res, err = runDebug(t, suppressed)
	if err != nil {
		t.Fatalf("run with S3=1: %v\n", err)
	}
	if dbg != "" {
		t.Errorf("debugging stream with S3=1: want it empty: got\n%s\n", dbg)
	}
	if out != value {
		t.Errorf("output stream with S3=1: S3 suppresses the message, not the copy\n\twant %q\n\t got %q\n", value, out)
	}
	if res.Errors != 0 {
		t.Errorf("S5 with S3=1: want 0: got %d\n", res.Errors)
	}
}

// TestDebugErrorAbortsTheConstruction covers what an error does to the process
// around it, which is the behaviour the phrase "detects all errors and prints a
// message at every occurrence" is standing on.
//
// An error does not stop the process. The construction being performed is
// abandoned and given a null value, a subsidiary message says which one, the
// count in S5 goes up, and the scan carries on with the text after it. That is
// how one bad line produces one message rather than a cascade, and it is why
// the exit status distinguishes a process that reported errors from one that
// died.
//
// The error is raised inside a macro so that the context print-out has to walk
// out through it: the innermost frame is the call of MCSET, the next is the
// replacement text it was written in, and the outermost is the source.
func TestDebugErrorAbortsTheConstruction(t *testing.T) {
	const source = `MCSKIP MT,<>
MCDEF SHOW AS <MCSET P1 = A
done>
SHOW
tail
`
	out, dbg, res, err := runDebug(t, source)
	if !errors.Is(err, ml1.ErrProcessErrors) {
		t.Errorf("run: want ErrProcessErrors: got %v\n", err)
	}
	// the MCSET was abandoned, and everything after it was not: the rest of the
	// replacement text and the rest of the source are both there.
	if want := "done\ntail\n"; out != want {
		t.Errorf("output stream:\n\twant %q\n\t got %q\n", want, out)
	}
	wantMentions(t, "debugging stream", dbg,
		// the prologue, which is what S5 actually counts
		"Error(s)",
		`Argument 2 has illegal value, viz "A"`,
		// the context, innermost first
		"detected in",
		"macro MCSET",
		"called from",
		"line 1 of macro SHOW",
		"line 4 of source text",
		// the subsidiary message: which construction was given a null value
		"aborted due to above error",
	)
	if got, want := res.Errors, 1; got != want {
		t.Errorf("S5: want %d: got %d\n", want, got)
	}
	if res.Fatal {
		t.Errorf("fatal: want false; the process ran to the end and reported\n")
	}
	if got, want := res.ExitStatus(), 254; got != want {
		t.Errorf("exit status: want %d: got %d\n", want, got)
	}
}

// TestDebugEndOfProcessReport covers S18, the only thing this stream says about
// a process that went right.
//
// Its two bits are independent and both start clear, which is why a clean run
// writes nothing here at all and why every golden file in the local corpus can
// assert an empty stream. Bit 2^1 asks for the statistics line and bit 2^0 for
// the list of currently defined constructions.
//
// The report is written after the process is over, so it is exempt from the S12
// quota. That is asserted here rather than with the quota itself, because it is
// a property of the report: a process that has already been cut off would
// otherwise lose the one summary that says how far it got.
func TestDebugEndOfProcessReport(t *testing.T) {
	// the source is the same in every case bar the setting, so that the two
	// bits are compared against each other and not against different work
	const template = `MCSKIP MT,<>
MCDEF GREET AS <hello>
MCSET S18=%d
GREET
`
	source := func(s18 int) string { return fmt.Sprintf(template, s18) }

	// S18=0, the initial setting: nothing is written.
	if _, dbg, _, err := runDebug(t, source(0)); err != nil {
		t.Fatalf("S18=0: %v\n", err)
	} else if dbg != "" {
		t.Errorf("S18=0: want an empty stream: got\n%s\n", dbg)
	}

	// bit 2^1 alone: the statistics line and nothing else. The two numbers are
	// the ones Result reports, so this also pins that they are read back from
	// the same place the message takes them from.
	_, dbg, res, err := runDebug(t, source(2))
	if err != nil {
		t.Fatalf("S18=2: %v\n", err)
	}
	// the two numbers come back in Result as well, taken from the same places
	// the finalisation code takes them from, so the line the reader sees and the
	// value a caller gets are the same fact.
	wantMentions(t, "S18=2", dbg,
		"At end of process:",
		fmt.Sprintf("%d line", res.Lines),
		fmt.Sprintf("%d call", res.Calls),
	)
	if strings.Contains(dbg, "Macros are") {
		t.Errorf("S18=2: want no list of constructions: got\n%s\n", dbg)
	}

	// bit 2^0 alone: the version of the machine independent logic and the
	// constructions, grouped by type. Both groups this source fills are checked,
	// GREET under the macros and the < of MCSKIP under the skips, because a list
	// that printed only its headings would otherwise pass.
	_, dbg, _, err = runDebug(t, source(1))
	if err != nil {
		t.Fatalf("S18=1: %v\n", err)
	}
	wantMentions(t, "S18=1", dbg,
		"Version ", "Stops are", "Macros are", "GREET", "Warnings are", "Inserts are", "Skips are\n\n<",
	)
	if strings.Contains(dbg, "At end of process:") {
		t.Errorf("S18=1: want no statistics line: got\n%s\n", dbg)
	}

	// both bits: both parts, once each.
	_, both, _, err := runDebug(t, source(3))
	if err != nil {
		t.Fatalf("S18=3: %v\n", err)
	}
	stats, list := strings.Index(both, "At end of process:"), strings.Index(both, "Macros are")
	if stats < 0 || list < 0 {
		t.Fatalf("S18=3: want both parts of the report: got\n%s\n", both)
	}
	if n := strings.Count(both, "At end of process:"); n != 1 {
		t.Errorf("S18=3: want the statistics line once: got %d\n", n)
	}
	// The order is the source's, and the source we run is the 1986 one: LOHALT
	// writes the statistics and then calls PRENV to list the constructions.
	// Appendix AA describes the other order, "preceded by a list of the
	// currently defined constructions", which is what the later CKQ logic does
	// — the same version skew BURNDOWN.md records elsewhere. This asserts what
	// the engine we have must do, so that a change to it is deliberate.
	if stats > list {
		t.Errorf("S18=3: want the statistics before the list, as LOHALT writes them: got\n%s\n", both)
	}

	// the report is not charged against S12, so a quota of nothing left still
	// gets it written and the process still ends cleanly.
	const exhausted = `MCSKIP MT,<>
MCDEF GREET AS <hello>
MCSET S18=3
MCSET S12=0
GREET
`
	out, dbg, res, err := runDebug(t, exhausted)
	if err != nil {
		t.Fatalf("S12=0 with a report to write: %v\n", err)
	}
	if want := "hello\n"; out != want {
		t.Errorf("S12=0: output stream:\n\twant %q\n\t got %q\n", want, out)
	}
	wantMentions(t, "S12=0", dbg, "At end of process:", "Macros are")
	if res.Fatal {
		t.Errorf("S12=0: fatal: want false; the report does not spend the quota\n")
	}
}
