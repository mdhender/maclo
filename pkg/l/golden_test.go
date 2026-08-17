// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package l_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/l"
	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/sema"
	"github.com/mdhender/maclo/pkg/l/token"
)

// update rewrites the goldens in pkg/l/testdata.
//
// The flag is registered in this package only, so "go test ./... -update"
// still fails elsewhere with "flag provided but not defined" and the
// documented invocation is "go test ./pkg/l -update".
var update = flag.Bool("update", false, "rewrite the goldens in pkg/l/testdata")

// The corpus is ours. Every input under testdata was written from the L
// manual, and nothing is copied out of the L source of ML/I - that file is
// under a licence that does not permit redistribution, and the whole of
// .references is gitignored for the same reason.
//
// Per case NAME:
//
//	NAME.l    the input
//	NAME.lst  the statement listing, compared byte for byte
//	NAME.sym  the symbol table; optional
//	NAME.err  the diagnostics; optional, and a missing one means "none"
//
// A missing .err reading as "there must be no diagnostics" is the discipline
// pkg/ml1/testdata/local already uses, and it is what stops a case quietly
// acquiring a warning nobody looked at.

func TestGolden(t *testing.T) {
	inputs, err := filepath.Glob("testdata/*.l")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Fatal("no inputs in pkg/l/testdata")
	}
	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".l")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			result := l.Parse(src)

			var listing, symbols bytes.Buffer
			if err := ast.WriteListing(&listing, result.Program); err != nil {
				t.Fatal(err)
			}
			if err := sema.WriteListing(&symbols, result.Table); err != nil {
				t.Fatal(err)
			}

			base := strings.TrimSuffix(input, ".l")
			compare(t, base+".lst", listing.Bytes(), false)
			compare(t, base+".sym", symbols.Bytes(), true)
			compare(t, base+".err", diagnostics(result.Errs), true)
		})
	}
}

// compare checks one golden. optional says a missing file means the output
// must be empty rather than that the case is not checked.
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
		// Refusing to create a golden is what stops current behaviour from
		// becoming the specification by accident. Create the file yourself,
		// empty, and then -update will fill it in.
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
	t.Errorf("%s differs\n--- want ---\n%s\n--- got ---\n%s\n%s",
		path, want, got, firstDifference(want, got))
}

// diagnostics renders the whole report, warnings included, so that a case
// cannot pick up a warning without a golden changing.
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

func firstDifference(want, got []byte) string {
	w, g := strings.Split(string(want), "\n"), strings.Split(string(got), "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		a, b := "", ""
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			b = g[i]
		}
		if a != b {
			return "first difference at line " + itoa(i+1) + ":\n want " + a + "\n  got " + b
		}
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// TestRoundTrip renders every case back to L, re-runs the front end over what
// it wrote, and requires the same text a second time.
//
// It is what makes the canonical form load bearing rather than decorative: a
// listing that cannot be read back is a listing that has quietly lost
// something.
func TestRoundTrip(t *testing.T) {
	inputs, err := filepath.Glob("testdata/*.l")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range inputs {
		t.Run(strings.TrimSuffix(filepath.Base(input), ".l"), func(t *testing.T) {
			src, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			first := l.Parse(src)
			var a bytes.Buffer
			if err := ast.WriteSource(&a, first.Program); err != nil {
				t.Fatal(err)
			}

			second := l.Parse(a.Bytes())
			for _, e := range second.Errs {
				if e.Severity == token.SevError && !hadError(first.Errs) {
					t.Errorf("re-reading the listing found %s: %s", e.Pos, e.Msg)
				}
			}
			var b bytes.Buffer
			if err := ast.WriteSource(&b, second.Program); err != nil {
				t.Fatal(err)
			}
			if a.String() != b.String() {
				t.Errorf("the listing does not read back as itself\n%s", firstDifference(a.Bytes(), b.Bytes()))
			}
		})
	}
}

func hadError(errs token.Errors) bool { return errs.HasErrors() }
