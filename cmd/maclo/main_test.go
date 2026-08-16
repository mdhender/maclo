// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/ml1"
)

// TestChooseEngine covers the three-way decision that is the whole point of
// this command: a name it was built with, a path to something it was not, and
// a word that is neither.
func TestChooseEngine(t *testing.T) {
	// a path, which must be taken as one even though it is not embedded
	file := filepath.Join(t.TempDir(), "elsewhere.lwl")
	if err := os.WriteFile(file, []byte("LOWL"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var job ml1.Job
	if err := (&options{engine: file}).chooseEngine(&job); err != nil {
		t.Errorf("a path: %v", err)
	} else if job.LOWLSource != file {
		t.Errorf("a path: want LOWLSource %s, got %q (Engine %q)", file, job.LOWLSource, job.Engine)
	}

	// a word that is neither: the diagnostic must say what the binary has,
	// because the likely cause is a version this build does not carry
	job = ml1.Job{}
	err := (&options{engine: "ml1zzz"}).chooseEngine(&job)
	if err == nil {
		t.Fatalf("an unknown name: want an error, got nil")
	}
	if !strings.Contains(err.Error(), "ml1zzz") {
		t.Errorf("want the error to name what was asked for, got %v", err)
	}
	if !strings.Contains(err.Error(), "built in") && !strings.Contains(err.Error(), "no engines") {
		t.Errorf("want the error to say what this binary has, got %v", err)
	}

	// a name this binary carries, when it carries any
	engines := ml1.Engines()
	if len(engines) == 0 {
		// The zero-engine build is a legitimate one, so this is a skip. What
		// must still hold is that asking for nothing is refused rather than
		// silently falling back to a file search: maclo runs what it was
		// built with.
		job = ml1.Job{}
		if err := (&options{}).chooseEngine(&job); err == nil {
			t.Errorf("no engines and no --engine: want an error, got nil")
		} else if !strings.Contains(err.Error(), "no ML/I engine") {
			t.Errorf("want the no-engine explanation, got %v", err)
		}
		t.Skip("no engines are embedded in this build; run: go run ./cmd/fetchtestdata")
	}

	job = ml1.Job{}
	if err := (&options{engine: engines[0].Name}).chooseEngine(&job); err != nil {
		t.Errorf("an embedded name: %v", err)
	} else if job.Engine != engines[0].Name || job.LOWLSource != "" {
		t.Errorf("an embedded name: want Engine %s and no path, got Engine %q LOWLSource %q",
			engines[0].Name, job.Engine, job.LOWLSource)
	}

	// and no --engine at all takes the newest
	job = ml1.Job{}
	if err := (&options{}).chooseEngine(&job); err != nil {
		t.Errorf("no --engine: %v", err)
	} else if job.Engine != ml1.DefaultEngine() {
		t.Errorf("no --engine: want the default (%s), got %q", ml1.DefaultEngine(), job.Engine)
	}
}

// TestParseFlags covers the options that differ from cmd/ml1, since being a
// different command line is what this program is for. --out repeats, and the
// input files are whatever is left.
func TestParseFlags(t *testing.T) {
	var out, errOut bytes.Buffer
	opt, code, done := parse(
		[]string{"--engine", "ml1ajb", "--out", "a.txt", "--out", "b.txt", "--workspace", "9000", "x.ml1", "y.ml1"},
		&out, &errOut)
	if done {
		t.Fatalf("want the command to continue, got status %d (%s)", code, errOut.String())
	}
	if opt.engine != "ml1ajb" {
		t.Errorf("engine: want ml1ajb, got %q", opt.engine)
	}
	if len(opt.outputs) != 2 || opt.outputs[0] != "a.txt" || opt.outputs[1] != "b.txt" {
		t.Errorf("out: want two files in order, got %v", opt.outputs)
	}
	if opt.workspace != 9000 {
		t.Errorf("workspace: want 9000, got %d", opt.workspace)
	}
	if len(opt.inputs) != 2 || opt.inputs[0] != "x.ml1" {
		t.Errorf("inputs: want the two remaining arguments, got %v", opt.inputs)
	}

	// an unknown flag ends the command rather than being taken as a file
	out.Reset()
	errOut.Reset()
	if _, code, done := parse([]string{"--nope"}, &out, &errOut); !done || code == 0 {
		t.Errorf("an unknown flag: want the command to stop with a non-zero status, got done=%v code=%d", done, code)
	}
}

// TestListEnginesReportsEmptyBuild checks the message a build with nothing in
// it gives. It is the first thing anyone will see after `go install`, so it has
// to explain itself rather than print an empty list.
func TestListEnginesReportsEmptyBuild(t *testing.T) {
	var out, errOut bytes.Buffer
	code := listEngines(&out, &errOut)

	if len(ml1.Engines()) == 0 {
		if code == 0 {
			t.Errorf("no engines: want a non-zero status, got 0")
		}
		if !strings.Contains(errOut.String(), "no ML/I engine") {
			t.Errorf("no engines: want an explanation on stderr, got %q", errOut.String())
		}
		if out.Len() != 0 {
			t.Errorf("no engines: want nothing on stdout, got %q", out.String())
		}
		return
	}

	if code != 0 {
		t.Errorf("with engines: want status 0, got %d", code)
	}
	if !strings.Contains(out.String(), ml1.DefaultEngine()) {
		t.Errorf("with engines: want the default listed, got %q", out.String())
	}
	if !strings.Contains(out.String(), "(default)") {
		t.Errorf("with engines: want the default marked, got %q", out.String())
	}
}

// TestRunReportsAMissingInput checks what a bad command line says, and in
// which order.
//
// The engine is settled before any input file is opened, so a build with no
// engine says so even when the command line is also wrong. That ordering is
// deliberate rather than incidental: without an engine nothing this program
// does can succeed, so it is the more useful thing to be told first, and
// fixing it is a rebuild rather than a retype.
func TestRunReportsAMissingInput(t *testing.T) {
	var out, errOut bytes.Buffer
	missing := filepath.Join(t.TempDir(), "nope.ml1")

	status := run([]string{missing}, strings.NewReader(""), &out, &errOut)
	if status != 255 {
		t.Errorf("missing input: want status 255, got %d", status)
	}

	if len(ml1.Engines()) == 0 {
		if !strings.Contains(errOut.String(), "no ML/I engine") {
			t.Errorf("no engines: want the engine reported before the input, got %q", errOut.String())
		}
		return
	}
	if !strings.Contains(errOut.String(), "nope.ml1") {
		t.Errorf("want the missing file named, got %q", errOut.String())
	}
}
