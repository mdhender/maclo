// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mdhender/maclo/pkg/ml1"
)

// A library call must leave no trace of itself.
//
// Every stage of the pipeline can write a debug artifact — scanner_tokens.txt,
// ast_listing.txt, asm_listing.txt, asm_symtab.txt and the rest — and each one
// goes to the *current directory*, because the program they were written for is
// cmd/lasm and that is where a developer wants them. The plumbing that keeps
// them out of ml1.Run is opt-in on the caller's side: cst.ParseBuffer instead of
// cst.Parse, assembler.Options{} with Listings unset, and a nil vm.Streams.Trace.
//
// Opt-in plumbing is the kind that gets forgotten. A new stage that writes its
// listing unconditionally would pass every other test in this package — the
// golden corpora compare buffers, and a file dropped into the working directory
// does not change what is in them — so this is the test that would notice.
//
// It is also the reason to run the whole thing from a directory of its own: the
// assertion is not "no file called X" but "no file at all", which is the only
// form that catches an artifact nobody has thought of yet.

// TestRunWritesNothing runs the processor from an empty directory, with the
// process's own streams redirected, and requires that it produced no file and
// printed nothing.
//
// The four jobs are the paths that reach different amounts of the engine: a
// clean run, a run that asks for a listing and the end of process report, a run
// that reports errors, and a run that is aborted. All four assemble the LOWL
// source, which is where most of the artifacts come from, and the last two also
// run the diagnostic and finalisation code.
func TestRunWritesNothing(t *testing.T) {
	// resolved before the working directory changes, because both of these
	// walk up from it
	source := lowlSource()
	requireEngine(t)

	dir := t.TempDir()
	restore := chdir(t, dir)
	defer restore()

	stdout, stderr, drain := captureStdStreams(t)
	defer drain()

	for _, tc := range []struct {
		id     int
		name   string
		source string
		listed bool // give the job a listing stream, as -l does
	}{
		{1, "a clean run", "the quick brown fox\n", false},
		{2, "a listing and the end of process report", "MCSET S20=1\nMCSET S18=3\nsome text\n", true},
		{3, "a run that reports errors", "MCSKIP MT,<>\nMCDEF SHOW AS <MCSET P1 = A\ndone>\nSHOW\n", false},
		{4, "a run that is aborted", "hello\nMCSET S10=9\nafter\n", false},
	} {
		var out, dbg, lst bytes.Buffer
		job := ml1.Job{
			Inputs:     []ml1.Input{ml1.StringInput("silence.ml1", tc.source)},
			Outputs:    []io.Writer{&out},
			Debug:      &dbg,
			Workspace:  ml1.DefaultWorkspace,
			DebugWidth: ml1.NeverWrap,
			LOWLSource: source,
		}
		if tc.listed {
			job.Listing = &lst
		}

		// What each of these runs produced belongs to debug_test.go and
		// fatal_test.go; two of them end in an error on purpose, and all that
		// matters here is that the engine really ran. Nothing but a missing
		// engine would mean it had not.
		if _, err := ml1.Run(job); errors.Is(err, ml1.ErrNoEngineSource) {
			t.Fatalf("%d: %s: %v\n", tc.id, tc.name, err)
		}
		if left := entriesIn(t, dir); len(left) != 0 {
			t.Errorf("%d: %s: wrote %v into the working directory\n"+
				"\ta library call must not; see the Options/Trace/Listings plumbing\n",
				tc.id, tc.name, left)
			// clean up, so that one leak is not reported by every case after it
			for _, name := range left {
				_ = os.RemoveAll(filepath.Join(dir, name))
			}
		}
	}

	// A job the engine will not even start still must not create the file it
	// was pointed at, or print its complaint.
	_, err := ml1.Run(ml1.Job{
		Inputs:     []ml1.Input{ml1.StringInput("silence.ml1", "")},
		Outputs:    []io.Writer{io.Discard},
		Debug:      io.Discard,
		LOWLSource: filepath.Join(dir, "no-such-engine.lwl"),
	})
	if !errors.Is(err, ml1.ErrNoEngineSource) {
		t.Errorf("5: a job with no engine: want ErrNoEngineSource: got %v\n", err)
	}
	if left := entriesIn(t, dir); len(left) != 0 {
		t.Errorf("5: a job with no engine: wrote %v into the working directory\n", left)
	}

	drain()
	if s := stdout.String(); s != "" {
		t.Errorf("the standard output: want nothing: got %q\n", s)
	}
	if s := stderr.String(); s != "" {
		t.Errorf("the standard error: want nothing: got %q\n", s)
	}
}

// entriesIn lists what is in a directory, sorted, so that a leak is reported by
// name rather than by count.
func entriesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("%s: %v\n", dir, err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// chdir moves the process into dir and returns the function that puts it back.
//
// This changes state the whole test binary shares, so nothing in this package
// may call t.Parallel while it is in effect. (t.Chdir would do this for us, and
// arrived in go 1.24; this module is on 1.20.)
func chdir(t *testing.T, dir string) func() {
	t.Helper()
	was, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v\n", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v\n", dir, err)
	}
	return func() {
		if err := os.Chdir(was); err != nil {
			t.Fatalf("chdir %s: %v\n", was, err)
		}
	}
}

// captureStdStreams replaces the process's standard output and standard error
// with pipes and returns the buffers they are being read into, along with the
// function that restores them and waits for the readers.
//
// It has to be the file descriptors rather than an io.Writer the code was
// handed, because what is being checked is precisely the case of code that was
// handed nothing and printed anyway. Draining happens on a goroutine so that a
// run which printed more than a pipe holds cannot deadlock the test it is
// failing.
//
// The returned function is safe to call twice: the deferred call is the one
// that runs when the test fails early.
func captureStdStreams(t *testing.T) (stdout, stderr *bytes.Buffer, drain func()) {
	t.Helper()
	outWas, errWas := os.Stdout, os.Stderr
	stdout, stderr = &bytes.Buffer{}, &bytes.Buffer{}

	// both pipes are made before either stream is replaced, so that a failure
	// here cannot leave the test binary writing into a pipe nothing reads
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v\n", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		_, _ = outR.Close(), outW.Close()
		t.Fatalf("pipe: %v\n", err)
	}

	var done []chan struct{}
	read := func(r *os.File, into *bytes.Buffer) {
		finished := make(chan struct{})
		done = append(done, finished)
		go func() {
			defer close(finished)
			_, _ = io.Copy(into, r)
			_ = r.Close()
		}()
	}
	read(outR, stdout)
	read(errR, stderr)
	os.Stdout, os.Stderr = outW, errW

	drained := false
	return stdout, stderr, func() {
		if drained {
			return
		}
		drained = true
		_ = os.Stdout.Close()
		_ = os.Stderr.Close()
		os.Stdout, os.Stderr = outWas, errWas
		for _, finished := range done {
			<-finished
		}
	}
}
