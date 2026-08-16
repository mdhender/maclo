// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/lowl/assembler"
	"github.com/mdhender/maclo/pkg/lowl/ast"
	"github.com/mdhender/maclo/pkg/lowl/cst"
)

// TestLOWLTEST runs the LOWL kernel conformance program, version L4A.
//
// This is the only end-to-end test of the opcode kernel. op_test.go checks
// opcodes one at a time against expectations built by hand; LOWLTEST is a real
// program that exercises them against each other, which is how it would notice
// a regression in something like GOADD or the two stacks. It is worth having
// even though a passing run is not proof of much on its own: two bugs — RL
// storing an address rather than a distance, and CSS popping one link instead
// of clearing the stack — survived both a clean assembly and a passing
// LOWLTEST before they were found.
//
// The program cannot be committed here, so the test skips when it is absent,
// the same way the upstream corpus does in pkg/ml1. The skip is keyed on the
// file rather than on an environment variable or a build tag, so that it
// expires by itself the moment the source is fetched.
func TestLOWLTEST(t *testing.T) {
	source, ok := lowlTestSource(t)
	if !ok {
		return
	}

	nodes := cst.ParseBuffer(source)
	for _, node := range nodes {
		if node.Error != nil {
			t.Fatalf("cst: %d:%d: %v\n", node.Line, node.Col, node.Error)
		}
	}
	tree, err := ast.Parse(nodes)
	if err != nil {
		t.Fatalf("ast: %v\n", err)
	}
	// silent: no trace stream and no listings, because a test must not write
	// files into the directory it was run from.
	m, err := assembler.Assemble(tree, assembler.Options{})
	if err != nil {
		t.Fatalf("assemble: %v\n", err)
	}

	// LOWLTEST finishes well inside the default, but the default is a runaway
	// guard rather than a budget, and a test one instruction away from tripping
	// it would be reporting the wrong thing.
	m.Registers.Cycles = 1_000_000

	var out, msg bytes.Buffer
	if err := m.Run(&out, &msg); err != nil {
		t.Fatalf("run: %v\n", err)
	}

	lines := strings.Split(strings.TrimSuffix(msg.String(), "\n"), "\n")
	checkNoFailures(t, lines)
	checkEverySectionReported(t, lines)
	checkCharactersRepresented(t, lines)
}

// checkNoFailures looks for the marker every LOWLTEST failure message carries.
// The program has around sixty of them and they all begin the same way, so this
// one test stands in for all of them and needs no list to keep up to date.
func checkNoFailures(t *testing.T, lines []string) {
	t.Helper()
	for n, line := range lines {
		if strings.HasPrefix(line, "+++") {
			t.Errorf("lowltest: line %d: %s\n", n+1, line)
		}
	}
}

// wantSections is how many sections LOWLTEST announces. Counting them is what
// catches a run that stopped early: a machine that halts in the middle writes
// no failure message, so checkNoFailures alone would call that a pass.
const wantSections = 14

// checkEverySectionReported pairs each announcement with its result.
//
// LOWLTEST announces a section with a line of leading dots and then reports
// either OK or found. Anything else in that position — a failure message, or
// nothing at all because the machine stopped — is a fault. The banner also
// starts with dots, hence the match on what follows them rather than on the
// dots alone.
func checkEverySectionReported(t *testing.T, lines []string) {
	t.Helper()
	sections := 0
	for n, line := range lines {
		if !isAnnouncement(line) {
			continue
		}
		sections++
		result, ok := nextNonBlank(lines, n+1)
		if !ok {
			t.Errorf("lowltest: %s: no result before the end of the stream\n", line)
			continue
		}
		if result != "OK" && result != "found" {
			t.Errorf("lowltest: %s: want OK or found, got %q\n", line, result)
		}
	}
	if sections != wantSections {
		t.Errorf("lowltest: sections: want %d, got %d\n", wantSections, sections)
	}
}

func isAnnouncement(line string) bool {
	rest, ok := strings.CutPrefix(line, "...")
	if !ok {
		return false
	}
	lower := strings.ToLower(rest)
	return strings.HasPrefix(lower, "testing") || strings.HasPrefix(lower, "searching")
}

func nextNonBlank(lines []string, from int) (string, bool) {
	for ; from < len(lines); from++ {
		if strings.TrimSpace(lines[from]) != "" {
			return lines[from], true
		}
	}
	return "", false
}

// checkCharactersRepresented compares what the last section of LOWLTEST leaves
// to the eye.
//
// It writes a run of characters one at a time and then a literal saying what
// they should have looked like, for a human to compare. The machine can do that
// comparison, and it is worth doing: this is the only part of the program that
// tests how characters are represented rather than how control flows, and the
// two lines go through the character output path a byte at a time.
func checkCharactersRepresented(t *testing.T, lines []string) {
	t.Helper()
	// the literal reads ".,;:()*/-+= TAB and quote sign", which is this
	want := []string{
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ 0123456789",
		".,;:()*/-+=\t\"",
	}
	for _, line := range want {
		if !containsLine(lines, line) {
			t.Errorf("lowltest: characters: %q not written\n", line)
		}
	}
}

func containsLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

// lowlTestSource reads LOWLTEST, or skips the test when it is not here.
func lowlTestSource(t *testing.T) ([]byte, bool) {
	t.Helper()
	name := filepath.Join(moduleRoot(), ".downloads", "lowltest", "ltestl4a.lwl")
	source, err := os.ReadFile(name)
	if err != nil {
		t.Skipf("%s: not fetched; run: go run ./cmd/fetchtestdata -corpus lowltest\n", name)
		return nil, false
	}
	return source, true
}

// moduleRoot finds the top of the repository, because the tests run from the
// package directory and the archives are unpacked at the top.
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
