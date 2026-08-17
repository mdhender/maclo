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
)

// TestRunWritesNothingUnasked is the rule this command exists on the right
// side of.
//
// cmd/lasm writes seven listings into whatever directory it was run from, and
// debug_artifacts_test.go had to grow a table to keep that honest. Here every
// listing goes to a path the caller named, so a run that asks for none must
// leave the directory it ran in exactly as it found it.
func TestRunWritesNothingUnasked(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "clean.l")
	write(t, source, cleanProgram)

	before := entries(t, dir)
	var out, errOut bytes.Buffer
	code, err := run(&config{source: source}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit %d on a clean program; diagnostics:\n%s", code, errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("wrote to stdout without being asked:\n%s", out.String())
	}
	if got := entries(t, dir); len(got) != len(before) {
		t.Errorf("left files behind: %v", got)
	}
}

// TestExitCode: an error fails the run and a warning does not. A name longer
// than the manual allows is worth saying and is not a reason to stop a build.
func TestExitCode(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want int
	}{
		{"clean", cleanProgram, 0},
		{"a warning only", warnProgram, 0},
		{"an error", errorProgram, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "case.l")
			write(t, source, tc.src)

			var out, errOut bytes.Buffer
			code, err := run(&config{source: source}, &out, &errOut)
			if err != nil {
				t.Fatal(err)
			}
			if code != tc.want {
				t.Errorf("exit %d, want %d; diagnostics:\n%s", code, tc.want, errOut.String())
			}
		})
	}
}

func TestListingsGoWhereTheyAreAsked(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "case.l")
	write(t, source, cleanProgram)
	listing := filepath.Join(dir, "out.lst")

	var out, errOut bytes.Buffer
	if _, err := run(&config{source: source, listing: listing, symbols: "-", quiet: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(listing)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "PRGSTART") {
		t.Errorf("the listing file does not hold the program:\n%s", got)
	}
	if !strings.Contains(out.String(), "TOTAL") {
		t.Errorf("the symbol table did not reach stdout:\n%s", out.String())
	}
}

func TestMissingSource(t *testing.T) {
	var out, errOut bytes.Buffer
	if _, err := run(&config{}, &out, &errOut); err == nil {
		t.Error("no --source was accepted")
	}
	if _, err := run(&config{source: filepath.Join(t.TempDir(), "nope.l")}, &out, &errOut); err == nil {
		t.Error("a source that does not exist was accepted")
	}
}

// TestMaxErrors caps the report without hiding the count.
func TestMaxErrors(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "case.l")
	write(t, source, errorProgram)

	var out, errOut bytes.Buffer
	if _, err := run(&config{source: source, maxErrors: 1}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "not shown") {
		t.Errorf("the capped diagnostics were not counted:\n%s", errOut.String())
	}
}

const cleanProgram = `PRGSTART

SECTION VARS,//ONE VARIABLE//
        DEC TOTAL INIT 0
ENDSECT VARS

SECTION MAIN,//AND ONE LOOP//
[BEGIN] SET TOTAL = TOTAL+1
        GO TO BEGIN
ENDSECT MAIN

PRGEND
`

const warnProgram = `PRGSTART

SECTION VARS,//A NAME LONGER THAN THE MANUAL ALLOWS//
        DEC OVERLONG INIT 0
ENDSECT VARS

SECTION MAIN,//AND ONE LOOP//
[BEGIN] SET OVERLONG = 1
        GO TO BEGIN
ENDSECT MAIN

PRGEND
`

const errorProgram = `PRGSTART

SECTION VARS,//ONE VARIABLE//
        DEC TOTAL INIT 0
ENDSECT VARS

SECTION MAIN,//TWO BRANCHES THAT NAME NOTHING//
[BEGIN] SET TOTAL = MISSNG
        GO TO NOWHER
        GO TO BEGIN
ENDSECT MAIN

PRGEND
`

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range list {
		names = append(names, e.Name())
	}
	return names
}
