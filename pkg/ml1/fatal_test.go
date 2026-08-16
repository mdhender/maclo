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

// The four fatal conditions of appendix AA.4.1, and the shape of what they
// write.
//
// These are the errors the host detects rather than the ones ML/I detects, and
// for a long time they were reported in a shape of this port's own: one line
// carrying a Go sentinel the user has never heard of, with no error count, no
// context, and nothing to say the process had stopped. What AA describes, and
// what the reference implementation writes, is an ordinary ML/I error
// print-out, so that a fatal condition reads like every other diagnostic on
// the stream.
//
// The assertions are on substrings and on their order, because the order is
// the thing that was missing: the prologue that S5 counts, the message, the
// context saying where the process had reached, and the line saying it stopped
// there. The wording is appendix AA's, which is why none of it is a golden
// file — see the note at the top of debug_test.go.
//
// Every case here was run against the reference implementation first, apart
// from the rewind failure, which cannot be: the oracle buffers its standard
// input and rewinds it successfully, so the condition is unreachable there.
// That one is asserted from AA.4.1.3 alone.
//
// One difference from the oracle runs is expected and is not asserted against.
// The prologue is PRERR's, and PRERR is one of the three literals in the 1986
// LOWL source that CKQ writes with one newline fewer, so our print-out has a
// blank line the oracle's does not. docs/explanation/running-ml1-on-the-lowl-vm.md
// records the skew; the assertions below step around it the way the rest of the
// suite does.

// runFatal runs one source and hands back both streams, the way runDebug does,
// but returns the error rather than failing on it: an aborted process is what
// these tests are about.
func runFatal(t *testing.T, job ml1.Job, source string) (out, dbg string, res ml1.Result, err error) {
	t.Helper()
	var o, d bytes.Buffer
	if job.Inputs == nil {
		job.Inputs = []ml1.Input{ml1.StringInput("fatal.ml1", source)}
	}
	if job.Outputs == nil {
		job.Outputs = []io.Writer{&o}
	}
	job.Debug = &d
	job.Workspace = ml1.DefaultWorkspace
	job.DebugWidth = ml1.NeverWrap
	job.LOWLSource = lowlSource()

	res, err = ml1.Run(job)
	if errors.Is(err, ml1.ErrNoEngineSource) {
		t.Skipf("%v; run: go run ./cmd/fetchtestdata\n", err)
	}
	return o.String(), d.String(), res, err
}

// wantInOrder checks that each of want appears on the stream, and that they
// appear in the order given.
//
// The order is what these tests are for. Every part of a fatal print-out could
// be present and the print-out still be wrong: the context belongs after the
// message it explains, and the line that says the process stopped belongs
// after everything it is the consequence of.
func wantInOrder(t *testing.T, what, stream string, want ...string) {
	t.Helper()
	rest, ok := stream, true
	for i, s := range want {
		at := strings.Index(rest, s)
		if at < 0 {
			if strings.Contains(stream, s) {
				t.Errorf("%s: want %q after %q\n", what, s, want[i-1])
			} else {
				t.Errorf("%s: want it to mention %q\n", what, s)
			}
			ok = false
			break
		}
		rest = rest[at+len(s):]
	}
	if !ok {
		t.Errorf("%s was:\n%s\n", what, stream)
	}
}

// TestFatalIllegalStream is AA.4.1.2, and it is the case that shows the whole
// shape at once.
//
// S10 is read afresh before every character, so the value takes effect on the
// character after the macro that set it: the context says line 3 and not line
// 2, which is where the assignment is. The oracle says line 3 as well.
//
// The context print-out is the part that cannot be written from Go — it walks
// ML/I's own stack of environments — so this is also the test that the machine
// really is being sent through the program's error path rather than being
// stopped where the condition was noticed.
func TestFatalIllegalStream(t *testing.T) {
	const source = `hello
MCSET S10=9
after
`
	out, dbg, res, err := runFatal(t, ml1.Job{}, source)

	if !errors.Is(err, ml1.ErrIllegalStream) {
		t.Errorf("run: want ErrIllegalStream: got %v\n", err)
	}
	if !res.Fatal {
		t.Errorf("fatal: want true; the process did not reach the end of its input\n")
	}
	if got, want := res.ExitStatus(), 255; got != want {
		t.Errorf("exit status: want %d: got %d\n", want, got)
	}
	// the prologue is what §6.3 says S5 counts, and counting it is half of
	// what PRERR does
	if got, want := res.Errors, 1; got != want {
		t.Errorf("S5: want %d: got %d\n", want, got)
	}

	wantInOrder(t, "debugging stream", dbg,
		"Error(s)",
		`S10 has illegal value, viz "9"`,
		"detected in",
		"line 3 of source text",
		"Process aborted due to above error",
	)
	// nothing of the Go error belongs on the stream. The sentinel is for a
	// caller to test with errors.Is and reads as noise to anyone else.
	if strings.Contains(dbg, "ml1:") {
		t.Errorf("debugging stream: want no Go error text: got:\n%s\n", dbg)
	}

	// the text before the assignment was written and the text after it was
	// not: a fatal condition stops the process where it happened.
	if want := "hello\n"; out != want {
		t.Errorf("output stream:\n\twant %q\n\t got %q\n", want, out)
	}
}

// TestFatalQuotaBoundary is AA.4.1.1, and it is about where exactly the quota
// runs out.
//
// S12 is a count of lines the debugging stream will still accept and the
// process is abandoned when it goes negative, which is one line further than
// "when it reaches zero". With S4 at one an MCNOTE writes two lines, so a quota
// of four covers two of them exactly and a quota of three does not: the second
// message is written whole, and the line it ends on is the one that goes
// negative. Both halves were checked against the oracle.
//
// The print-out has no context print-out in it, and that is not an omission:
// printing one would need more lines of the stream than there are.
func TestFatalQuotaBoundary(t *testing.T) {
	const source = `MCSET S4=1
MCSET S12=%d
MCNOTE one
MCNOTE two
tail
`
	t.Run("exactly enough", func(t *testing.T) {
		out, dbg, res, err := runFatal(t, ml1.Job{}, fmt.Sprintf(source, 4))
		if err != nil {
			t.Fatalf("run: %v\n", err)
		}
		// S4 suppresses the context, so every byte here came from this source
		if want := "\none\n\ntwo\n"; dbg != want {
			t.Errorf("debugging stream:\n\twant %q\n\t got %q\n", want, dbg)
		}
		if want := "tail\n"; out != want {
			t.Errorf("output stream:\n\twant %q\n\t got %q\n", want, out)
		}
		if res.Errors != 0 {
			t.Errorf("S5: want 0, a quota that held is not an error: got %d\n", res.Errors)
		}
	})

	t.Run("one short", func(t *testing.T) {
		out, dbg, res, err := runFatal(t, ml1.Job{}, fmt.Sprintf(source, 3))
		if !errors.Is(err, ml1.ErrDebugQuota) {
			t.Errorf("run: want ErrDebugQuota: got %v\n", err)
		}
		if got, want := res.ExitStatus(), 255; got != want {
			t.Errorf("exit status: want %d: got %d\n", want, got)
		}
		if got, want := res.Errors, 1; got != want {
			t.Errorf("S5: want %d: got %d\n", want, got)
		}

		wantInOrder(t, "debugging stream", dbg,
			"\none\n",
			"\ntwo\n", // the message that ran the quota out is written whole
			"Error(s)",
			"Debugging file lines quota exhausted",
			"Process aborted due to above error",
		)
		if strings.Contains(dbg, "detected in") {
			t.Errorf("debugging stream: want no context print-out; there are no lines left to write one:\n%s\n", dbg)
		}
		if strings.Contains(dbg, "ml1:") {
			t.Errorf("debugging stream: want no Go error text: got:\n%s\n", dbg)
		}
		if out != "" {
			t.Errorf("output stream: want it empty, the process stopped before the last line: got %q\n", out)
		}
	})
}

// TestFatalRewindFailure is AA.4.1.3.
//
// A value of S10 between 101 and 105 asks for the stream to be repositioned at
// its start, and an Input that cannot be read a second time — which is what a
// pipe is like — cannot do it. The message says only that it failed, so the
// error the caller gets keeps the stream number and the reason.
func TestFatalRewindFailure(t *testing.T) {
	const source = `one
MCSET S10=101
two
`
	job := ml1.Job{Inputs: []ml1.Input{ml1.StreamInput("-", strings.NewReader(source))}}
	out, dbg, res, err := runFatal(t, job, source)

	if !errors.Is(err, ml1.ErrCannotRewind) {
		t.Errorf("run: want ErrCannotRewind: got %v\n", err)
	}
	if got, want := res.ExitStatus(), 255; got != want {
		t.Errorf("exit status: want %d: got %d\n", want, got)
	}
	wantInOrder(t, "debugging stream", dbg,
		"Error(s)",
		"Cannot rewind input stream",
		"detected in",
		"line 3 of source text",
		"Process aborted due to above error",
	)
	if want := "one\n"; out != want {
		t.Errorf("output stream:\n\twant %q\n\t got %q\n", want, out)
	}
}

// TestFatalOutputWriteError is AA.4.1.4, whose message names the file that
// could not be written.
//
// It is also the one of the four that is detected in MDOUCH rather than in
// MDREAD, so it is what says the error path is reached from wherever the
// condition arises and not just from the one subroutine.
func TestFatalOutputWriteError(t *testing.T) {
	const source = `hello
`
	// a second output stream that fails on the first character. S21 is 1 to
	// begin with, so only stream 1 is written to; the source turns stream 2 on
	// and the failure happens on the character after.
	job := ml1.Job{Outputs: []io.Writer{io.Discard, failingWriter{}}}
	_, dbg, res, err := runFatal(t, job, "MCSET S21=3\n"+source)

	if err == nil {
		t.Fatalf("run: want an error: got nil\n")
	}
	if !res.Fatal {
		t.Errorf("fatal: want true\n")
	}
	wantInOrder(t, "debugging stream", dbg,
		"Error(s)",
		"Error while writing to output file 2",
		"detected in",
		"of source text",
		"Process aborted due to above error",
	)
}

// failingWriter is an output stream that cannot be written to.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("device not ready")
}
