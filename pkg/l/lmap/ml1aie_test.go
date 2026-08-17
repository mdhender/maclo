// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package lmap_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/l"
	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/lmap"
)

// The real thing.
//
// pkg/l/lmap/testdata is written from the manual, which means it is written by
// the same person who wrote the code and tests what that person thought of.
// ml1aie.l is 2,510 lines of L written in 1971 by somebody who had never seen
// this L-map, and ml1aig.lwl is what an L-map of their own made of it. So this
// is the case with a specification and an answer key, and it is the one to run
// all through the work.
//
// It cannot be committed -- the L source of ML/I is copyright P.J. Brown and
// R.D. Eager and its licence does not permit redistributing a machine readable
// copy -- so everything here is keyed on the file being on disk, and the skip
// expires by itself when it is fetched.

// The two files this reads, relative to the module root.
const (
	// lSource is the L source as the archive ships it. It has one defect, and
	// TestML1AIE in pkg/l is what asserts it is still exactly one.
	lSource = ".downloads/lml1/ml1aie.l"

	// lSourceFixed is that file with the defect corrected. The archive does
	// not contain it: correcting a copyright source is something a reader does
	// in an editor, not something this repository can ship, so this is keyed
	// on separately and says how to make it when it is not there.
	lSourceFixed = ".downloads/lml1/ml1aie2.l"

	// answerKey is the published LOWL of the version nearest the L source
	// above, which is what an L-map like this one produced from it.
	answerKey = ".downloads/lowlml1aig/ml1aig.lwl"
)

// howToFix says what to do about the one defect. The name of the label is not
// written here for the same reason it is not written in TestML1AIE: it is a
// line of a source that may not be redistributed, and the front end names it
// when it is run.
const howToFix = `%s: not present.

The L source of ML/I has one defect in it: a TEST branches to a label whose
declaration is spelled with a letter too many, so the branch has no target.
pkg/l reports it, TestML1AIE asserts that it is still the only one, and a back
end refuses to emit an engine from a program with an undefined name in it.

The archive ships the file with the defect. To map it, copy it and correct the
declaration to the spelling the branch uses:

    cp %[2]s %[1]s
    go test ./pkg/l -run TestML1AIE -v     # names the label and both lines
`

// TestRefusesAnUndefinedName is the property the correction exists for.
//
// A front end reports and keeps going, because a listing of the whole file is
// what a reader wants. A back end cannot: a branch to a label nothing declares
// maps into a branch to a label nothing defines, and the assembler would then
// report a fault of the generated text rather than of the source it came from.
func TestRefusesAnUndefinedName(t *testing.T) {
	src, ok := read(t, lSource, "run: go run ./cmd/fetchtestdata -corpus lml1")
	if !ok {
		return
	}
	if result := l.Parse(src); !result.HasErrors() {
		t.Errorf("%s resolves; it is the file with the defect, so it should not", lSource)
	}
}

// TestMapML1AIE maps the real source and holds the answer to the published one.
func TestMapML1AIE(t *testing.T) {
	src, ok := read(t, lSourceFixed, "")
	if !ok {
		return
	}
	result := l.Parse(src)
	if result.HasErrors() {
		t.Fatalf("%s does not resolve:\n%s", lSourceFixed, result.Errs.Sorted())
	}

	prog, errs := lmap.Map(result.Program, result.Table)
	t.Run("maps", func(t *testing.T) {
		// Every statement the L-map cannot map reports, so a clean run is a
		// statement about coverage rather than about the absence of crashes.
		for _, e := range errs.Sorted() {
			t.Errorf("%s: %s", e.Pos, e.Msg)
		}
		t.Logf("%d statements of L mapped", result.Summary().Statements)
	})

	var buf bytes.Buffer
	if err := prog.WriteLOWL(&buf); err != nil {
		t.Fatal(err)
	}
	generated := buf.Bytes()

	t.Run("assembles", func(t *testing.T) {
		// This is the check with teeth. The assembler works out where every
		// word lands and refuses an RL whose distance disagrees with it, an
		// EXIT numbered past its subroutine's exits, an operand of the wrong
		// shape, and every name nothing defines.
		m, err := assemble(generated)
		if err != nil {
			t.Fatalf("the generated LOWL does not assemble: %v", err)
		}
		t.Logf("%d lines of LOWL, %d words of core, entry at %d",
			bytes.Count(generated, []byte("\n")), m.PC, m.Registers.Start)
	})

	published, ok := read(t, answerKey, "run: go run ./cmd/fetchtestdata -corpus lowlml1aig")
	if !ok {
		return
	}

	t.Run("tables", func(t *testing.T) { checkTables(t, generated, published, stopAt(result.Program)) })
	t.Run("subroutines", func(t *testing.T) { checkSubroutines(t, generated, published) })
}

// checkTables holds the data SECTIONs to the published ones, word for word.
//
// This is the part of the output that is a translation rather than a choice.
// A table item has one spelling, and the distance in an RL has one right
// answer, so a divergence here is a mistake rather than a difference of
// opinion -- and a wrong distance is not a wrong number, it is a pointer into
// the middle of a string.
//
// The comparison stops where the L source stops describing the tables. What
// follows is the chain of layout characters and the hash table, which chapter
// 6 of the manual leaves to the implementor: the two implementors chose
// different names for the entries and put them in a different order, and both
// are right.
func checkTables(t *testing.T, generated, published []byte, stop string) {
	mine := tableWords(generated, stop)
	theirs := tableWords(published, stop)
	if len(mine) == 0 || len(theirs) == 0 {
		t.Fatalf("no table words found: %d in the generated LOWL, %d in the published one", len(mine), len(theirs))
	}

	var wrong int
	for i := 0; i < len(mine) && i < len(theirs); i++ {
		if mine[i] != theirs[i] {
			wrong++
			if wrong <= 10 {
				t.Errorf("table word %d: the published LOWL has %q and this L-map made %q", i+1, theirs[i], mine[i])
			}
		}
	}
	if len(mine) != len(theirs) {
		t.Errorf("the tables are %d words long and the published ones are %d", len(mine), len(theirs))
	}
	if wrong > 10 {
		t.Errorf("and %d more words differ", wrong-10)
	}
	if !t.Failed() {
		t.Logf("%d table words, every one the same as the published LOWL", len(mine))
	}
}

// tableWords returns the items of the data SECTIONs, one string each, from the
// first hash chain entry to the last item the L source describes.
//
// Labels are dropped because a label emits no code: the two L-maps write them
// in different places on the line, and one of them numbers its own. What is
// left is the word and its operand, which is what lands in core.
func tableWords(source []byte, stop string) []string {
	var out []string
	started := false
	for _, line := range strings.Split(string(source), "\n") {
		if label := labelled.FindString(line); label != "" && stop != "" && strings.Trim(strings.TrimSpace(label), "[]") == stop {
			return out
		}
		line = strings.TrimSpace(labelled.ReplaceAllString(line, ""))
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		switch fields[0] {
		case "HASH":
			started = true
		case "CON", "STR", "RL", "NCH", "WTHS":
		case "THASH":
			// the hash table, which is the implementor's rather than the L
			// source's, and nothing the L source describes follows it
			return out
		default:
			continue
		}
		if !started {
			continue
		}
		// A length of one word is written both ways and means the same thing.
		out = append(out, strings.Replace(strings.Join(fields, " "), "OF(1*LCH)", "OF(LCH)", 1))
	}
	return out
}

// labelled matches a label at the start of a line, with or without a statement
// after it.
var labelled = regexp.MustCompile(`^\s*\[[A-Z0-9]+\]`)

// stopAt is the label the L source writes on its LAYCHAIN statement, which is
// where it stops describing the tables and the implementor starts.
//
// It is read off the tree rather than written down here, because it is a name
// the program chose. Everything from there on is the chain of layout
// characters and the hash table, which chapter 6 of the manual leaves open,
// and two implementors are entitled to lay them out differently.
func stopAt(prog *ast.Program) string {
	var name string
	ast.Inspect(prog, func(n ast.Node) bool {
		if t, ok := n.(*ast.LayChain); ok && name == "" && t.Label != nil {
			name = t.Label.Text
		}
		return true
	})
	return name
}

// checkSubroutines requires every subroutine of the MI-logic to be in the
// published LOWL under the same name.
//
// It is one directional on purpose. What the published LOWL has and this does
// not, and the other way about, is the MD-logic -- the code chapter 7 says the
// implementor writes -- and two implementors are entitled to split that
// differently. What is not a matter of opinion is that a subroutine the L
// source declares has to come out the other side.
func checkSubroutines(t *testing.T, generated, published []byte) {
	mine, theirs := subroutines(generated), subroutines(published)
	if len(mine) == 0 || len(theirs) == 0 {
		t.Fatalf("no subroutines found: %d in the generated LOWL, %d in the published one", len(mine), len(theirs))
	}

	var missing, extra []string
	for name := range mine {
		if !theirs[name] {
			missing = append(missing, name)
		}
	}
	for name := range theirs {
		if !mine[name] {
			extra = append(extra, name)
		}
	}
	for _, name := range missing {
		if strings.HasPrefix(name, "LO") {
			// the MD-logic this L-map wrote for itself
			continue
		}
		t.Errorf("%s is a subroutine here and not in the published LOWL", name)
	}
	t.Logf("%d subroutines, %d of them also in the published LOWL", len(mine), len(mine)-len(missing))
	if len(extra) != 0 {
		t.Logf("the published LOWL has %d this L-map does not: %s", len(extra), strings.Join(extra, " "))
	}
}

// subroutines returns the names a source declares with SUBR.
func subroutines(source []byte) map[string]bool {
	out := make(map[string]bool)
	for _, line := range strings.Split(string(source), "\n") {
		fields := strings.Fields(labelled.ReplaceAllString(line, ""))
		if len(fields) < 2 || fields[0] != "SUBR" {
			continue
		}
		out[strings.Split(fields[1], ",")[0]] = true
	}
	return out
}

// read loads one of the files this test needs, or skips.
//
// The skip is keyed on the file rather than on an environment variable or a
// build tag, so that it expires by itself once the file is there. A skip and a
// failure are different things, and which one this is depends only on what has
// been fetched.
func read(t *testing.T, name, how string) ([]byte, bool) {
	t.Helper()
	path := filepath.Join(moduleRoot(), name)
	src, err := os.ReadFile(path)
	if err != nil {
		if how == "" {
			t.Skipf(howToFix, path, filepath.Join(moduleRoot(), lSource))
		} else {
			t.Skipf("%s: not fetched; %s", path, how)
		}
		return nil, false
	}
	return src, true
}

// moduleRoot walks up to the directory holding go.mod, so that a path written
// from the root of the repository works from a package directory.
func moduleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
