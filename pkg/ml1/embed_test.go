// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ml1_test

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/mdhender/maclo/pkg/ml1"
)

// The embedded engines.
//
// How many there are is a property of the machine the binary was built on, not
// of this source, so these tests skip when there are none rather than fail: a
// clone that has not fetched anything is a legitimate build and has to stay
// one, which is the whole reason //go:embed points at a directory holding a
// tracked README rather than at a pattern that has to match.
//
// What is *not* conditional is the split between the two ways of naming an
// engine. cmd/ml1 leaves both Job fields empty and must keep searching the file
// system whatever the binary was built with, and TestEmbeddedIsNotConsultedByDefault
// is what stops that quietly changing.

func requireEmbedded(t *testing.T) []ml1.Engine {
	t.Helper()
	engines := ml1.Engines()
	if len(engines) == 0 {
		t.Skip("no engines are embedded in this build; run: go run ./cmd/fetchtestdata")
	}
	return engines
}

// TestEngines covers the listing and the order. The order is the policy — the
// newest engine is the default — so it is worth pinning even when a build
// carries only one.
func TestEngines(t *testing.T) {
	engines := requireEmbedded(t)

	for i, e := range engines {
		if e.Name == "" {
			t.Errorf("%d: empty name", i)
		}
		if filepath.Ext(e.Name) != "" {
			t.Errorf("%d: %s still has an extension; the name is what a user types", i, e.Name)
		}
		if e.Size <= 0 {
			t.Errorf("%s: size %d", e.Name, e.Size)
		}
		if !ml1.HasEngine(e.Name) {
			t.Errorf("%s: listed but HasEngine says no", e.Name)
		}
		if i > 0 && engines[i-1].Name <= e.Name {
			t.Errorf("want newest first: %s is not after %s", engines[i-1].Name, e.Name)
		}
	}

	if got, want := ml1.DefaultEngine(), engines[0].Name; got != want {
		t.Errorf("DefaultEngine: want the newest (%s), got %s", want, got)
	}
	if ml1.HasEngine("ml1zzz") {
		t.Errorf("HasEngine: want false for an engine nobody has built")
	}
}

// TestRunFromEmbeddedEngine is the case maclo relies on: a job that names no
// path at all and runs anyway, because the processor is inside the binary.
func TestRunFromEmbeddedEngine(t *testing.T) {
	engines := requireEmbedded(t)

	var out, dbg bytes.Buffer
	res, err := ml1.Run(ml1.Job{
		Inputs:  []ml1.Input{ml1.StringInput("embedded.ml1", "from the binary\n")},
		Outputs: []io.Writer{&out},
		Debug:   &dbg,
		Engine:  engines[0].Name,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, want := out.String(), "from the binary\n"; got != want {
		t.Errorf("output: want %q: got %q", want, got)
	}
	if res.Errors != 0 {
		t.Errorf("errors: want 0, got %d", res.Errors)
	}
}

// TestUnknownEngineIsRefused covers the name that is not there. It must not
// fall through to a file search and pick up whatever the working directory
// happens to hold, because that would make a typo run a different processor.
func TestUnknownEngineIsRefused(t *testing.T) {
	_, err := ml1.Run(ml1.Job{
		Inputs:  []ml1.Input{ml1.StringInput("probe.ml1", "")},
		Outputs: []io.Writer{io.Discard},
		Debug:   io.Discard,
		Engine:  "ml1zzz",
	})
	if !errors.Is(err, ml1.ErrNoEngineSource) {
		t.Errorf("want ErrNoEngineSource, got %v", err)
	}
}

// TestLOWLSourceWinsOverEngine pins which field is answered first. A path is
// how a user runs a version the binary was not built with, so it has to beat
// the built-in default rather than be shadowed by it.
func TestLOWLSourceWinsOverEngine(t *testing.T) {
	requireEngine(t) // this one needs the source on disk, not in the binary

	var out, dbg bytes.Buffer
	_, err := ml1.Run(ml1.Job{
		Inputs:     []ml1.Input{ml1.StringInput("both.ml1", "path wins\n")},
		Outputs:    []io.Writer{&out},
		Debug:      &dbg,
		LOWLSource: lowlSource(),
		Engine:     "ml1zzz", // would fail if it were consulted
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, want := out.String(), "path wins\n"; got != want {
		t.Errorf("output: want %q: got %q", want, got)
	}
}

// TestEmbeddedIsNotConsultedByDefault is the guarantee cmd/ml1 depends on.
//
// ml1 implements the operating instructions and finds its engine on disk. If
// Run started preferring an embedded engine when neither Job field was set,
// ml1 would silently stop honouring $ML1_LOWL_SOURCE and the per-user
// directory on any binary that happened to be built with a source in it —
// which is most of them, and none of the existing tests would have noticed.
func TestEmbeddedIsNotConsultedByDefault(t *testing.T) {
	requireEmbedded(t)

	empty := t.TempDir()
	t.Setenv(ml1.EngineEnv, filepath.Join(empty, "named.lwl"))
	t.Setenv(ml1.HomeEnv, empty)

	_, err := ml1.Run(ml1.Job{
		// neither LOWLSource nor Engine: the file search, and nothing else
		Inputs:  []ml1.Input{ml1.StringInput("probe.ml1", "")},
		Outputs: []io.Writer{io.Discard},
		Debug:   io.Discard,
	})
	if !errors.Is(err, ml1.ErrNoEngineSource) {
		t.Fatalf("want the file search to fail with ErrNoEngineSource, got %v\n"+
			"\tan embedded engine must not answer a job that named none", err)
	}
}
