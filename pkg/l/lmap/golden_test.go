// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package lmap_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/l"
	"github.com/mdhender/maclo/pkg/l/lmap"
	"github.com/mdhender/maclo/pkg/l/token"
)

// The flag is registered in this package only, so "go test ./... -update"
// still fails elsewhere with "flag provided but not defined" and the
// documented invocation is "go test ./pkg/l/lmap -update".
var update = flag.Bool("update", false, "rewrite the goldens in pkg/l/lmap/testdata")

// mdLogicMarker is the first line of the code L does not describe.
//
// Every program maps into the same MD-logic, so a golden that held it as well
// as the statements it is named for would be nine tenths the same file nine
// times over. The harness cuts there, and TestMDLogic is what holds the other
// half to its own golden.
const mdLogicMarker = "'the MD-logic: the code L leaves to the L-map'"

func TestGolden(t *testing.T) {
	inputs, err := filepath.Glob("testdata/*.l")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Fatal("no inputs in pkg/l/lmap/testdata")
	}
	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".l")
		t.Run(name, func(t *testing.T) {
			statements, _, errs := mapFile(t, input)
			base := strings.TrimSuffix(input, ".l")
			compare(t, base+".lwl", statements, false)
			compare(t, base+".err", diagnostics(errs), true)
		})
	}
}

// TestMDLogic holds the half of the output that is the same whatever was
// mapped. It is one case rather than a golden per program, and the program it
// maps is the one that asks for every part of it.
func TestMDLogic(t *testing.T) {
	_, mdLogic, errs := mapFile(t, "testdata/prelude.l")
	if errs.HasErrors() {
		t.Fatalf("prelude.l does not map:\n%s", errs.Sorted())
	}
	compare(t, "testdata/mdlogic.lwl", mdLogic, false)
}

// TestAssembles is the check the goldens cannot make.
//
// A golden says the output is what it was last time; it does not say the
// output is LOWL. Running the assembler over it does, and it does more than
// that: the assembler reports an RL whose distance disagrees with the layout
// it made, an EXIT whose number is beyond its subroutine's, and every name
// nothing defines. Those are exactly the mistakes an emitter makes.
//
// It runs over the one case that is a whole program. The others are missing
// the MI-logic the MD-logic calls -- there is no PRNUM in a program about
// CHARMATCH -- so their names would be undefined for a reason that is about
// the case rather than about the mapping.
func TestAssembles(t *testing.T) {
	statements, mdLogic, errs := mapFile(t, "testdata/prelude.l")
	if errs.HasErrors() {
		t.Fatalf("prelude.l does not map:\n%s", errs.Sorted())
	}
	if _, err := assemble(append(append([]byte{}, statements...), mdLogic...)); err != nil {
		t.Errorf("the generated LOWL does not assemble: %v", err)
	}
}

// mapFile runs the front end and the L-map over one file and splits the
// output where the MD-logic starts.
func mapFile(t *testing.T, path string) (statements, mdLogic []byte, errs token.Errors) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := l.Parse(src)
	if result.HasErrors() {
		// A case that does not resolve is a broken case: the L-map is not what
		// is being tested when the L is wrong.
		t.Fatalf("%s does not resolve:\n%s", path, result.Errs.Sorted())
	}
	prog, errs := lmap.Map(result.Program, result.Table)
	var buf bytes.Buffer
	if err := prog.WriteLOWL(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()
	cut := bytes.Index(out, []byte(mdLogicMarker))
	if cut < 0 {
		t.Fatalf("%s: the MD-logic marker is not in the output", path)
	}
	// back up over the comment's own line and the blank line above it
	for cut > 0 && out[cut-1] != '\n' {
		cut--
	}
	for cut > 1 && out[cut-2] == '\n' {
		cut--
	}
	return out[:cut], out[cut:], errs
}

// compare is the same contract pkg/l's harness has: an optional golden that is
// missing means the output must be empty, and a required one that is missing
// is refused rather than created, so that current behaviour cannot become the
// specification by accident.
func compare(t *testing.T, path string, got []byte, optional bool) {
	t.Helper()
	want, err := os.ReadFile(path)
	switch {
	case err == nil:
	case !os.IsNotExist(err):
		t.Fatal(err)
	case optional:
		if len(got) != 0 {
			t.Errorf("%s does not exist, so the output must be empty; got:\n%s", path, got)
		}
		return
	default:
		t.Fatalf("%s does not exist: create it empty before running -update", path)
	}

	if bytes.Equal(got, want) {
		return
	}
	if *update {
		if err := os.WriteFile(path, got, 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s updated", path)
		return
	}
	t.Errorf("%s differs\n%s", path, firstDifference(want, got))
}

// diagnostics renders the L-map's own reports. The stage is left out because
// there is only one that can appear here.
func diagnostics(errs token.Errors) []byte {
	if len(errs) == 0 {
		return nil
	}
	var b bytes.Buffer
	for _, e := range errs.Sorted() {
		b.WriteString(e.Pos.String())
		b.WriteString(": ")
		b.WriteString(e.Severity.String())
		b.WriteString(": ")
		b.WriteString(e.Msg)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

// firstDifference names the line the two texts part company on. A whole
// generated engine is too long to read in a failure message, and the line
// number is what a reader needs to go and look.
func firstDifference(want, got []byte) string {
	a, b := strings.Split(string(want), "\n"), strings.Split(string(got), "\n")
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return "first difference at line " + itoa(i+1) + ":\n want " + a[i] + "\n  got " + b[i]
		}
	}
	switch {
	case len(a) < len(b):
		return "the output has " + itoa(len(b)-len(a)) + " more lines, the first being:\n  got " + b[len(a)]
	case len(b) < len(a):
		return "the output has " + itoa(len(a)-len(b)) + " fewer lines, the first missing being:\n want " + a[len(b)]
	}
	return "the two differ in trailing bytes rather than in any line"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
