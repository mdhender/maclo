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

// TestExitCodes. The three are distinct so that a script can tell "your L is
// wrong" from "your command line is wrong", which are answers a reader acts on
// differently. run adds the two ML/I returns on top of them, and those are far
// away in the numbering for the same reason.
func TestExitCodes(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		src  string
		want int
	}{
		{"a clean program", []string{"check"}, cleanProgram, exitOK},
		{"a warning is not a failure", []string{"check"}, warnProgram, exitOK},
		{"an error is", []string{"check"}, errorProgram, exitErrors},
		{"a listing of a broken program still fails", []string{"list"}, errorProgram, exitErrors},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := sourceFile(t, tc.src)
			var out, errOut bytes.Buffer
			if got := dispatch(append(tc.args, source, "--quiet"), &out, &errOut); got != tc.want {
				t.Errorf("exit %d, want %d; stderr:\n%s", got, tc.want, errOut.String())
			}
		})
	}
}

// TestUsageErrors. Everything here is the caller's mistake rather than the
// program's, and all of it has to be told apart from an L source that is
// merely wrong.
func TestUsageErrors(t *testing.T) {
	source := sourceFile(t, cleanProgram)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"no command at all", nil},
		{"a command that does not exist", []string{"compile", source}},
		{"no source", []string{"check"}},
		{"a source that does not exist", []string{"check", filepath.Join(t.TempDir(), "nope.l")}},
		{"two sources", []string{"check", source, source}},
		{"a flag that does not exist", []string{"check", source, "--verbose"}},
		{"--out on a command with no listing", []string{"check", source, "--out", "x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if got := dispatch(tc.args, &out, &errOut); got != exitUsage {
				t.Errorf("exit %d, want %d", got, exitUsage)
			}
			if errOut.Len() == 0 {
				t.Error("nothing was said about it")
			}
			if out.Len() != 0 {
				t.Errorf("a usage error wrote to stdout:\n%s", out.String())
			}
		})
	}
}

// TestWritesNothingUnasked is the rule pkg/l is built to keep, checked from
// the outside.
//
// cmd/lasm writes seven listings into whatever directory it was run from, and
// debug_artifacts_test.go had to grow a table to keep that honest. Here every
// listing goes to a path the caller named, so no command may leave a file
// behind, and none may write to stdout unless that is where its listing was
// sent.
func TestWritesNothingUnasked(t *testing.T) {
	for _, name := range []string{"check", "summary", "list", "symbols", "source", "lowl", "run"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "case.l")
			write(t, source, cleanProgram)
			before := entries(t, dir)

			args := []string{name, source, "--quiet"}
			if name == "lowl" || name == "run" {
				// neither takes --quiet: they either produce an engine or say
				// why they could not, and there is nothing to be quiet about
				args = []string{name, source}
			}
			var out, errOut bytes.Buffer
			dispatch(args, &out, &errOut)

			if got := entries(t, dir); len(got) != len(before) {
				t.Errorf("left files behind: %v", got)
			}
		})
	}
}

func TestListingGoesWhereItIsAsked(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "case.l")
	write(t, source, cleanProgram)
	out := filepath.Join(dir, "case.lst")

	var stdout, stderr bytes.Buffer
	if code := dispatch([]string{"list", source, "--out", out, "--quiet"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("the listing went to stdout as well:\n%s", stdout.String())
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "PRGSTART") {
		t.Errorf("the file does not hold the program:\n%s", got)
	}
}

// TestFlagsMayFollowTheFileName. The standard flag package stops at the first
// non-flag, so a command that parsed once would take `macl list f.l --quiet`
// as a request to check a file called --quiet.
func TestFlagsMayFollowTheFileName(t *testing.T) {
	source := sourceFile(t, cleanProgram)
	for _, args := range [][]string{
		{"list", "--quiet", source},
		{"list", source, "--quiet"},
	} {
		var out, errOut bytes.Buffer
		if code := dispatch(args, &out, &errOut); code != exitOK {
			t.Fatalf("%v: exit %d; stderr:\n%s", args, code, errOut.String())
		}
		if errOut.Len() != 0 {
			t.Errorf("%v: --quiet was not honoured:\n%s", args, errOut.String())
		}
	}
}

// TestSummaryIsStable. The counts come out of maps, and a report that ordered
// them by iteration would be a different report on every run. This package
// learnt that from its own golden files and the lesson is cheap to keep.
func TestSummaryIsStable(t *testing.T) {
	source := sourceFile(t, cleanProgram)
	var first bytes.Buffer
	for i := range 5 {
		var out, errOut bytes.Buffer
		if code := dispatch([]string{"summary", source, "--quiet"}, &out, &errOut); code != exitOK {
			t.Fatalf("exit %d; stderr:\n%s", code, errOut.String())
		}
		if i == 0 {
			first = out
			continue
		}
		if out.String() != first.String() {
			t.Fatalf("run %d differs from the first:\n%s\nwant:\n%s", i, out.String(), first.String())
		}
	}
	if !strings.Contains(first.String(), "statements") {
		t.Errorf("that is not a summary:\n%s", first.String())
	}
}

// TestRunWantsSomethingToProcess. An L program is not a job on its own: it is
// the processor, and what a processor needs is a text to process. Saying so is
// a command line error rather than a failure of the program, because the
// program has not been looked at yet.
func TestRunWantsSomethingToProcess(t *testing.T) {
	source := sourceFile(t, cleanProgram)
	var out, errOut bytes.Buffer
	if code := dispatch([]string{"run", source}, &out, &errOut); code != exitUsage {
		t.Fatalf("exit %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut.String(), "--source") {
		t.Errorf("the message does not say what is missing:\n%s", errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("run wrote to stdout:\n%s", out.String())
	}
}

// TestRunRefusesAProgramThatDoesNotResolve. The front end reports and keeps
// going, which is right for a listing; a back end cannot, because a branch to
// a label nothing declares becomes a branch to a label nothing defines and the
// complaint would then be about the generated text.
func TestRunRefusesAProgramThatDoesNotResolve(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "case.l")
	write(t, source, errorProgram)
	text := filepath.Join(dir, "case.ml1")
	write(t, text, "nothing to expand\n")

	var out, errOut bytes.Buffer
	if code := dispatch([]string{"run", source, "--source", text}, &out, &errOut); code != exitErrors {
		t.Fatalf("exit %d, want %d; stderr:\n%s", code, exitErrors, errOut.String())
	}
	if !strings.Contains(errOut.String(), "MISSNG") {
		t.Errorf("the report does not name what is undeclared:\n%s", errOut.String())
	}
}

// TestLowlWritesTheEngine. macl lowl is how the answer is looked at, and the
// answer has to be something an assembler would take.
func TestLowlWritesTheEngine(t *testing.T) {
	source := sourceFile(t, cleanProgram)
	var out, errOut bytes.Buffer
	if code := dispatch([]string{"lowl", source}, &out, &errOut); code != exitOK {
		t.Fatalf("exit %d, want %d; stderr:\n%s", code, exitOK, errOut.String())
	}
	for _, want := range []string{"PRGST", "DCL     TOTAL", "PRGEN"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the engine has no %q in it:\n%s", want, out.String())
		}
	}
	if errOut.Len() != 0 {
		t.Errorf("lowl wrote to stderr:\n%s", errOut.String())
	}
}

// TestHelpAndVersionGoToStdout, because they were asked for. A usage message
// printed because the command line was wrong is a diagnostic and goes to
// stderr; the same text printed because someone typed `macl help` is the
// answer and goes to stdout.
func TestHelpAndVersionGoToStdout(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"version"}} {
		var out, errOut bytes.Buffer
		if code := dispatch(args, &out, &errOut); code != exitOK {
			t.Errorf("%v: exit %d", args, code)
		}
		if out.Len() == 0 {
			t.Errorf("%v: said nothing", args)
		}
		if errOut.Len() != 0 {
			t.Errorf("%v: wrote to stderr:\n%s", args, errOut.String())
		}
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

func sourceFile(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "case.l")
	write(t, path, text)
	return path
}

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
