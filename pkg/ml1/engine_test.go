// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ml1_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/ml1"
)

// The engine is a runtime dependency, so where it is looked for is behaviour
// rather than plumbing.
//
// Until this search existed the only place ml1.Run would find a processor was
// .downloads under the working directory, which is a developer's checkout and
// nothing else: an installed binary run from anywhere at all reported
// ErrNoEngineSource no matter what the machine had on it. These tests are what
// keep that from coming back, and TestEngineFoundInHome is the one that says
// it in the form a user would notice.

// TestEngineDirHonoursHome covers the override, which is the only part of
// EngineDir that is the same on every platform. The rest of it is per-platform
// convention, so it is checked for being usable rather than for being a
// particular string.
func TestEngineDirHonoursHome(t *testing.T) {
	t.Setenv(ml1.HomeEnv, filepath.Join("some", "where"))
	if got, want := ml1.EngineDir(), filepath.Join("some", "where"); got != want {
		t.Errorf("with %s set: want %s: got %s", ml1.HomeEnv, want, got)
	}

	t.Setenv(ml1.HomeEnv, "")
	dir := ml1.EngineDir()
	if dir == "" {
		t.Fatalf("with %s unset: want a per-user directory, got nothing", ml1.HomeEnv)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("want an absolute directory, got %s", dir)
	}
	// the engine goes in a directory of our own, not loose in the user's
	if filepath.Base(dir) != "ml1" {
		t.Errorf("want a directory named ml1, got %s", dir)
	}
}

// TestEnginePaths pins the search order. It matters that the two overrides come
// first: a machine with an engine installed must still do what -s or the
// environment says, or there is no way to run a different version of ML/I.
func TestEnginePaths(t *testing.T) {
	named := filepath.Join("named", "by", "hand.lwl")
	t.Setenv(ml1.EngineEnv, named)
	t.Setenv(ml1.HomeEnv, filepath.Join("per", "user"))

	paths := ml1.EnginePaths()
	if len(paths) < 3 {
		t.Fatalf("want the overrides and the checkout paths, got %v", paths)
	}
	if paths[0] != named {
		t.Errorf("want $%s first, got %s", ml1.EngineEnv, paths[0])
	}
	if want := filepath.Join("per", "user", ml1.EngineFile); paths[1] != want {
		t.Errorf("want the per-user directory second (%s), got %s", want, paths[1])
	}
	if want := filepath.Join(".downloads", "lowlml1", ml1.EngineFile); paths[2] != want {
		t.Errorf("want the checkout layout third (%s), got %s", want, paths[2])
	}

	// An unset variable must drop out of the list rather than contribute an
	// empty name: os.ReadFile("") fails with a message about no such file,
	// which would put a blank line in the diagnostic the search prints.
	t.Setenv(ml1.EngineEnv, "")
	t.Setenv(ml1.HomeEnv, "")
	for _, p := range ml1.EnginePaths() {
		if p == "" {
			t.Errorf("want no empty entries, got %v", ml1.EnginePaths())
			break
		}
	}
}

// TestEngineFoundInHome is the installed-binary case: an engine that is only in
// the per-user directory, a Job that names no source, and a working directory
// with no checkout in it. That combination used to fail.
func TestEngineFoundInHome(t *testing.T) {
	requireEngine(t)

	source, err := os.ReadFile(lowlSource())
	if err != nil {
		t.Fatalf("read the engine: %v", err)
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ml1.EngineFile), source, 0644); err != nil {
		t.Fatalf("install the engine: %v", err)
	}

	t.Setenv(ml1.EngineEnv, "")
	t.Setenv(ml1.HomeEnv, home)

	var out, dbg bytes.Buffer
	res, err := ml1.Run(ml1.Job{
		// LOWLSource is deliberately empty: the whole point is that the
		// search finds it without being told
		Inputs:  []ml1.Input{ml1.StringInput("home.ml1", "found\n")},
		Outputs: []io.Writer{&out},
		Debug:   &dbg,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := out.String(); got != "found\n" {
		t.Errorf("output: want %q: got %q", "found\n", got)
	}
	if res.Errors != 0 {
		t.Errorf("errors: want 0, got %d", res.Errors)
	}
}

// TestNoEngineNamesEverywhereItLooked covers the diagnostic. A search with
// several candidates that reports one of them reads as though that were the
// only place it could have been, which is what sends a reader looking in the
// wrong directory.
func TestNoEngineNamesEverywhereItLooked(t *testing.T) {
	empty := t.TempDir()
	t.Setenv(ml1.EngineEnv, filepath.Join(empty, "named.lwl"))
	t.Setenv(ml1.HomeEnv, empty)

	_, err := ml1.Run(ml1.Job{
		Inputs:  []ml1.Input{ml1.StringInput("probe.ml1", "")},
		Outputs: []io.Writer{io.Discard},
		Debug:   io.Discard,
	})
	if err == nil {
		t.Fatalf("want ErrNoEngineSource, got nil (is there an engine in the working directory?)")
	}
	for _, want := range ml1.EnginePaths() {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("want the message to name %s, got %v", want, err)
		}
	}
}
