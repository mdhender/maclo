// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1_test

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/maloquacious/ml_i/pkg/ml1"
)

// TestValidate covers the checks that happen before the processor is entered.
// It never skips, so this package has real coverage while the engine is being
// written.
func TestValidate(t *testing.T) {
	var sink bytes.Buffer
	ok := func() ml1.Job {
		return ml1.Job{
			Inputs:  []ml1.Input{ml1.StringInput("a.ml1", "")},
			Outputs: []io.Writer{&sink},
			Debug:   &sink,
		}
	}

	if err := ok().Validate(); err != nil {
		t.Errorf("valid job: want nil error: got %v\n", err)
	}

	for _, tc := range []struct {
		id   int
		name string
		job  ml1.Job
		want error
	}{
		{1, "no inputs", ml1.Job{Outputs: []io.Writer{&sink}, Debug: &sink}, ml1.ErrNoInput},
		{2, "no outputs", ml1.Job{Inputs: []ml1.Input{ml1.StringInput("a", "")}, Debug: &sink}, ml1.ErrNoOutput},
		{3, "no debug", ml1.Job{Inputs: []ml1.Input{ml1.StringInput("a", "")}, Outputs: []io.Writer{&sink}}, ml1.ErrNoDebug},
		{4, "six inputs", func() ml1.Job {
			j := ok()
			for len(j.Inputs) < ml1.MaxInputs+1 {
				j.Inputs = append(j.Inputs, ml1.StringInput("x", ""))
			}
			return j
		}(), ml1.ErrTooManyInputs},
		{5, "five outputs", func() ml1.Job {
			j := ok()
			for len(j.Outputs) < ml1.MaxOutputs+1 {
				j.Outputs = append(j.Outputs, &sink)
			}
			return j
		}(), ml1.ErrTooManyOutputs},
		{6, "negative workspace", func() ml1.Job { j := ok(); j.Workspace = -1; return j }(), ml1.ErrWorkspace},
		{7, "negative debug width", func() ml1.Job { j := ok(); j.DebugWidth = -1; return j }(), ml1.ErrDebugWidth},
		{8, "input with no opener", func() ml1.Job {
			j := ok()
			j.Inputs = []ml1.Input{{Name: "a.ml1"}}
			return j
		}(), ml1.ErrNoInput},
		{9, "nil output", func() ml1.Job { j := ok(); j.Outputs = []io.Writer{nil}; return j }(), ml1.ErrNoOutput},
	} {
		if err := tc.job.Validate(); !errors.Is(err, tc.want) {
			t.Errorf("%d: %s: want %v: got %v\n", tc.id, tc.name, tc.want, err)
		}
		// Run must report the same thing, and must not reach the sentinel
		if _, err := ml1.Run(tc.job); !errors.Is(err, tc.want) {
			t.Errorf("%d: %s: run: want %v: got %v\n", tc.id, tc.name, tc.want, err)
		}
	}

	// a valid job reaches the engine, and the engine says what it could not
	// load rather than blaming the caller
	job := ok()
	job.LOWLSource = filepath.Join(t.TempDir(), "no-such-file.lwl")
	if _, err := ml1.Run(job); !errors.Is(err, ml1.ErrNoEngineSource) {
		t.Errorf("valid job: run: want %v: got %v\n", ml1.ErrNoEngineSource, err)
	}
}

// TestExitStatus pins the mapping the operating instructions describe.
func TestExitStatus(t *testing.T) {
	for _, tc := range []struct {
		id   int
		res  ml1.Result
		want int
	}{
		{1, ml1.Result{}, 0},
		{2, ml1.Result{Errors: 1}, 254},
		{3, ml1.Result{Fatal: true}, 255},
		{4, ml1.Result{Errors: 3, Fatal: true}, 255},
	} {
		if got := tc.res.ExitStatus(); got != tc.want {
			t.Errorf("%d: want %d: got %d\n", tc.id, tc.want, got)
		}
	}
}

// TestInputRewind covers the part of the stream model that a concatenating
// engine would have got wrong: a source may be read more than once, and one
// that cannot be says so.
func TestInputRewind(t *testing.T) {
	read := func(t *testing.T, in ml1.Input) (string, error) {
		t.Helper()
		rc, err := in.Open()
		if err != nil {
			return "", err
		}
		defer func() { _ = rc.Close() }()
		b, err := io.ReadAll(rc)
		return string(b), err
	}

	in := ml1.StringInput("a.ml1", "hello")
	for i := 1; i <= 2; i++ {
		if got, err := read(t, in); err != nil {
			t.Errorf("%d: bytes input: want nil error: got %v\n", i, err)
		} else if got != "hello" {
			t.Errorf("%d: bytes input: want %q: got %q\n", i, "hello", got)
		}
	}

	once := ml1.StreamInput("-", bytes.NewReader([]byte("hello")))
	if got, err := read(t, once); err != nil {
		t.Errorf("1: stream input: want nil error: got %v\n", err)
	} else if got != "hello" {
		t.Errorf("1: stream input: want %q: got %q\n", "hello", got)
	}
	if _, err := read(t, once); !errors.Is(err, ml1.ErrCannotRewind) {
		t.Errorf("2: stream input: want %v: got %v\n", ml1.ErrCannotRewind, err)
	}
}
