// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	// the command line from the comment on main, which interleaves options
	// and input files
	cfg, err := parseArgs([]string{"-v", "sets18.ml1", "foo.ml1", "-o", "foo.tmp_out", "-d", "foo.tmp_err"})
	if err != nil {
		t.Fatalf("parseArgs: want nil error, got %v", err)
	}
	if !cfg.version {
		t.Errorf("version: want true, got false")
	}
	if cfg.workspace != defaultWorkspace {
		t.Errorf("workspace: want %d, got %d", defaultWorkspace, cfg.workspace)
	}
	if cfg.listing != "" {
		t.Errorf("listing: want %q, got %q", "", cfg.listing)
	}
	if cfg.debug != "foo.tmp_err" {
		t.Errorf("debug: want %q, got %q", "foo.tmp_err", cfg.debug)
	}
	if got := strings.Join(cfg.output, ","); got != "foo.tmp_out" {
		t.Errorf("output: want %q, got %q", "foo.tmp_out", got)
	}
	if got := strings.Join(cfg.input, ","); got != "sets18.ml1,foo.ml1" {
		t.Errorf("input: want %q, got %q", "sets18.ml1,foo.ml1", got)
	}

	if cfg.source != "" {
		t.Errorf("source: want %q, got %q", "", cfg.source)
	}

	// -s names the LOWL source of ML/I, attached or separate
	for _, args := range [][]string{{"-s", "ml1ajb.lwl"}, {"-sml1ajb.lwl"}, {"-Sml1ajb.lwl"}} {
		if cfg, err = parseArgs(args); err != nil {
			t.Fatalf("parseArgs %v: want nil error, got %v", args, err)
		} else if cfg.source != "ml1ajb.lwl" {
			t.Errorf("parseArgs %v: source: want %q, got %q", args, "ml1ajb.lwl", cfg.source)
		}
	}

	// no arguments at all means the standard streams and the default workspace
	if cfg, err = parseArgs(nil); err != nil {
		t.Fatalf("parseArgs: want nil error, got %v", err)
	} else if cfg.version {
		t.Errorf("version: want false, got true")
	} else if cfg.workspace != defaultWorkspace {
		t.Errorf("workspace: want %d, got %d", defaultWorkspace, cfg.workspace)
	} else if cfg.listing != "" || cfg.debug != "" {
		t.Errorf("listing, debug: want %q, %q, got %q, %q", "", "", cfg.listing, cfg.debug)
	} else if len(cfg.output) != 0 || len(cfg.input) != 0 {
		t.Errorf("output, input: want 0, 0 files, got %d, %d", len(cfg.output), len(cfg.input))
	}

	// upper case option letters, attached values, and - as a file name
	if cfg, err = parseArgs([]string{"-W1000", "-L-", "-", "-O", "-"}); err != nil {
		t.Fatalf("parseArgs: want nil error, got %v", err)
	} else if cfg.workspace != 1000 {
		t.Errorf("workspace: want %d, got %d", 1000, cfg.workspace)
	} else if cfg.listing != "-" {
		t.Errorf("listing: want %q, got %q", "-", cfg.listing)
	} else if got := strings.Join(cfg.output, ","); got != "-" {
		t.Errorf("output: want %q, got %q", "-", got)
	} else if got := strings.Join(cfg.input, ","); got != "-" {
		t.Errorf("input: want %q, got %q", "-", got)
	}

	// the last -d, -l, and -w win
	if cfg, err = parseArgs([]string{"-d", "one", "-l", "two", "-w", "1", "-d", "three", "-l", "four", "-w", "2"}); err != nil {
		t.Fatalf("parseArgs: want nil error, got %v", err)
	} else if cfg.debug != "three" || cfg.listing != "four" || cfg.workspace != 2 {
		t.Errorf("debug, listing, workspace: want %q, %q, %d, got %q, %q, %d",
			"three", "four", 2, cfg.debug, cfg.listing, cfg.workspace)
	}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"unknown option", []string{"-x"}},
		{"value on -v", []string{"-vx"}},
		{"missing value", []string{"-o"}},
		{"workspace not a number", []string{"-w", "many"}},
		{"workspace not positive", []string{"-w", "0"}},
		{"six input files", []string{"a", "b", "c", "d", "e", "f"}},
		{"five output files", []string{"-o", "a", "-o", "b", "-o", "c", "-o", "d", "-o", "e"}},
	} {
		if _, err := parseArgs(tc.args); err == nil {
			t.Errorf("%s: want error, got nil", tc.name)
		}
	}
}

// TestParseArgsDebugWidth covers -c, which is an extension rather than part of
// the operating instructions. Zero is a legal value and means "never wrap", so
// it cannot be validated the way -w is.
func TestParseArgsDebugWidth(t *testing.T) {
	// the default applies when the option is absent
	if cfg, err := parseArgs(nil); err != nil {
		t.Fatalf("parseArgs: want nil error, got %v", err)
	} else if cfg.debugWidth != defaultDebugWidth {
		t.Errorf("debugWidth: want %d, got %d", defaultDebugWidth, cfg.debugWidth)
	}

	for _, tc := range []struct {
		id   int
		name string
		args []string
		want int
	}{
		{1, "attached", []string{"-c0"}, 0},
		{2, "separate", []string{"-c", "40"}, 40},
		{3, "upper case", []string{"-C", "40"}, 40},
		{4, "last wins", []string{"-c", "40", "-c", "0"}, 0},
	} {
		if cfg, err := parseArgs(tc.args); err != nil {
			t.Errorf("%d: %s: want nil error, got %v", tc.id, tc.name, err)
		} else if cfg.debugWidth != tc.want {
			t.Errorf("%d: %s: want %d, got %d", tc.id, tc.name, tc.want, cfg.debugWidth)
		}
	}

	for _, tc := range []struct {
		id   int
		name string
		args []string
	}{
		{5, "missing value", []string{"-c"}},
		{6, "not a number", []string{"-c", "wide"}},
		{7, "negative", []string{"-c", "-1"}},
	} {
		if _, err := parseArgs(tc.args); err == nil {
			t.Errorf("%d: %s: want error, got nil", tc.id, tc.name)
		}
	}
}

// TestRun drives the whole command from buffers. It does not build or exec
// anything, so it stays fast and works before the processor exists.
func TestRun(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "a.ml1")
	if err := os.WriteFile(source, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// a bad option reports the usage and fails
	var out, errOut bytes.Buffer
	if status := run([]string{"-x"}, strings.NewReader(""), &out, &errOut); status != 1 {
		t.Errorf("bad option: want status 1, got %d", status)
	} else if !strings.Contains(errOut.String(), "usage: ml1") {
		t.Errorf("bad option: want the usage on stderr, got %q", errOut.String())
	}

	// -v puts the banner on stderr, where it cannot be mistaken for output
	out.Reset()
	errOut.Reset()
	_ = run([]string{"-v", source}, strings.NewReader(""), &out, &errOut)
	if !strings.Contains(errOut.String(), "ml1: version ") {
		t.Errorf("version: want the banner on stderr, got %q", errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("version: want nothing on stdout, got %q", out.String())
	}

	// an input file that does not exist is reported before anything is run
	out.Reset()
	errOut.Reset()
	if status := run([]string{filepath.Join(dir, "nope.ml1")}, strings.NewReader(""), &out, &errOut); status != 255 {
		t.Errorf("missing input: want status 255, got %d", status)
	}

	// a job that is fine reaches the engine, which says what it could not load
	// and where to name it. The engine is the distributed LOWL source of ML/I,
	// which is not in this repository, so -s naming a file that is not there
	// is the same case as a machine that has not fetched it.
	out.Reset()
	errOut.Reset()
	if status := run([]string{"-s", filepath.Join(dir, "nope.lwl"), source}, strings.NewReader(""), &out, &errOut); status != 1 {
		t.Errorf("valid job: want status 1, got %d", status)
	} else if !strings.Contains(errOut.String(), "cannot read the LOWL source") {
		t.Errorf("valid job: want the missing source message, got %q", errOut.String())
	} else if !strings.Contains(errOut.String(), sourceEnv) {
		t.Errorf("valid job: want the message to name %s, got %q", sourceEnv, errOut.String())
	}
}

// TestJobStreams checks the wiring that has no equivalent in parseArgs: the
// name - resolving to the standard streams, and one name used twice
// resolving to a single writer rather than two buffers over one file.
func TestJobStreams(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "a.ml1")
	if err := os.WriteFile(source, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var out, errOut bytes.Buffer
	cfg, err := parseArgs([]string{source, "-o", "-", "-d", "-"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	job, closeAll, err := cfg.job(strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("job: %v", err)
	}
	defer func() { _ = closeAll() }()

	if len(job.Outputs) != 1 {
		t.Fatalf("outputs: want 1, got %d", len(job.Outputs))
	}
	// -o - and -d - both name the standard output, so they must be the very
	// same writer
	if job.Outputs[0] != job.Debug {
		t.Errorf("-o - and -d -: want one shared writer, got two")
	}
	if job.Listing != nil {
		t.Errorf("listing: want nil without -l, got %v", job.Listing)
	}
	if job.DebugWidth != defaultDebugWidth {
		t.Errorf("debugWidth: want %d, got %d", defaultDebugWidth, job.DebugWidth)
	}

	// two -o naming the same file resolve to one writer as well
	cfg, err = parseArgs([]string{source, "-o", filepath.Join(dir, "x"), "-o", filepath.Join(dir, "x")})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	job2, closeAll2, err := cfg.job(strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("job: %v", err)
	}
	defer func() { _ = closeAll2() }()
	if len(job2.Outputs) != 2 {
		t.Fatalf("outputs: want 2, got %d", len(job2.Outputs))
	}
	if job2.Outputs[0] != job2.Outputs[1] {
		t.Errorf("the same file named twice: want one shared writer, got two")
	}
}
