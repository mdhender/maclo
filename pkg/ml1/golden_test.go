// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1_test

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/maloquacious/ml_i/pkg/ml1"
)

// update rewrites the golden files of the corpus this repository owns. It
// refuses to touch the upstream corpus, whose golden files are the
// specification we are being measured against rather than a record of what we
// happen to produce.
//
// Because this flag is registered by one package only, "go test ./... -update"
// fails everywhere else with "flag provided but not defined". Run it as:
//
//	go test ./pkg/ml1 -update
var update = flag.Bool("update", false, "rewrite the golden files of the local corpus")

// corpus describes a directory of ML/I inputs with the expected output and
// debugging streams beside them.
//
// The suite from ml1.org.uk and the suite written for this repository differ
// only in where they live and how strictly they are judged, so one runner
// serves both.
type corpus struct {
	name    string // used in the test name and in messages
	dir     string // holds the inputs and the golden files
	prelude string // input stream 1, prepended to every run; "" for none

	// equal is the comparison this corpus is held to, and debugWidth is the
	// column its debugging stream is wrapped at. The two move together: the
	// upstream corpus gets the reference implementation's device width and
	// its tolerant comparison because we are conforming to its harness, while
	// ours gets unwrapped output and a byte-exact comparison because we write
	// both sides.
	equal      compareFunc
	debugWidth int

	// optional names the extensions whose golden file may be absent, in which
	// case that stream is required to be empty rather than left unchecked. A
	// corpus we did not write cannot say that: a stream with no golden beside
	// it there means the harness it came with never asked for one.
	optional map[string]bool

	// writable is true for the corpus this repository owns, the only one
	// -update is allowed to rewrite.
	writable bool

	// unstable names the cases whose debugging stream cannot be matched
	// portably, mapped to the reason. Their output stream is still compared.
	unstable map[string]string
}

// upstreamCorpus is the suite from ml1.org.uk. Its licence forbids
// redistributing a machine readable copy, so it is not in this repository;
// cmd/fetchtestdata puts it in place and the test skips until it does.
func upstreamCorpus() corpus {
	return corpus{
		name:       "upstream",
		dir:        filepath.Join(moduleRoot(), "testdata", "upstream", "tests-ac"),
		prelude:    "sets18.ml1",
		equal:      equalIgnoringSpaceChange,
		debugWidth: ml1.DefaultDebugWidth,
		unstable: map[string]string{
			// the test source says so itself: "Depth of nesting reached
			// before overflow occurs will vary between implementations"
			"overflow": "the depth reached before storage runs out is implementation dependent",
		},
	}
}

// localCorpus is the suite written for this repository: original inputs with
// golden files written by hand from the published manual, committed so that a
// fresh clone and a machine with no network still have real coverage.
func localCorpus() corpus {
	return corpus{
		name:       "local",
		dir:        filepath.Join("testdata", "local"),
		equal:      equalExact,
		debugWidth: ml1.NeverWrap,
		// a clean run writes nothing to either of these: S18 starts at zero, so
		// there is no end of process report, and S20 starts at zero, so there is
		// no listing.
		optional: map[string]bool{".err": true, ".lst": true},
		writable: true,
	}
}

func TestGoldenLocal(t *testing.T)    { runCorpus(t, localCorpus()) }
func TestGoldenUpstream(t *testing.T) { runCorpus(t, upstreamCorpus()) }

func runCorpus(t *testing.T, c corpus) {
	t.Helper()

	if _, err := os.Stat(c.dir); errors.Is(err, fs.ErrNotExist) {
		if c.writable {
			t.Fatalf("%s: %s is missing; it belongs to this repository\n", c.name, c.dir)
		}
		t.Skipf("%s: %s is not present; run: go run ./cmd/fetchtestdata\n", c.name, c.dir)
	} else if err != nil {
		t.Fatalf("%s: %v\n", c.name, err)
	}
	// -update must never touch this corpus, but making that a failure would
	// break "go test ./pkg/ml1 -update", which is the documented way to
	// refresh the local golden files. Skipping protects it just as well and
	// still says why.
	if *update && !c.writable {
		t.Skipf("%s: not rewritten by -update; these golden files are the specification\n", c.name)
	}

	cases := casesIn(t, c)
	if len(cases) == 0 {
		// a corpus that is present but empty must not look like success
		t.Fatalf("%s: no cases found in %s\n", c.name, c.dir)
	}

	// One probe, before the subtests, so an unwritten engine produces a
	// single clear skip instead of one per case. The skip keys on the
	// sentinel, so it goes away by itself when Run starts working.
	requireEngine(t)

	var prelude []ml1.Input
	if c.prelude != "" {
		data, err := os.ReadFile(filepath.Join(c.dir, c.prelude))
		if err != nil {
			t.Fatalf("%s: %v\n", c.name, err)
		}
		prelude = append(prelude, ml1.BytesInput(c.prelude, data))
	}

	for _, base := range cases {
		base := base // go 1.20: the loop variable is shared
		t.Run(base, func(t *testing.T) {
			runCase(t, c, base, prelude)
		})
	}
}

func runCase(t *testing.T, c corpus, base string, prelude []ml1.Input) {
	t.Helper()

	source, err := os.ReadFile(filepath.Join(c.dir, base+".ml1"))
	if err != nil {
		t.Fatalf("%s: %v\n", base, err)
	}

	inputs := make([]ml1.Input, 0, len(prelude)+1)
	inputs = append(inputs, prelude...)
	inputs = append(inputs, ml1.BytesInput(base+".ml1", source))
	inputs = append(inputs, extraStreams(t, c, base)...)

	// the listing is always offered, because asking for one is what the -l flag
	// does and S20 is what decides whether anything is written to it. A case
	// that never sets S20 leaves it empty, which is its own expectation.
	var out, dbg, lst bytes.Buffer
	job := ml1.Job{
		Inputs:     inputs,
		Outputs:    []io.Writer{&out},
		Debug:      &dbg,
		Listing:    &lst,
		Workspace:  ml1.DefaultWorkspace,
		DebugWidth: c.debugWidth,
		LOWLSource: lowlSource(),
	}

	// A run that reports errors, or that the processor aborted, is a valid
	// case: the upstream errors and overflow tests need exactly that, and the
	// golden files record what was written before it gave up. Anything else
	// is a fault in the harness rather than a result to compare.
	if _, err := ml1.Run(job); err != nil && !isProcessError(err) {
		t.Fatalf("%s: run: %v\n", base, err)
	}

	compare(t, c, base, ".out", out.String())
	// The listing is compared only where the corpus has something to say about
	// it. The upstream harness never asks for one, so a missing .lst there is
	// silence rather than a claim that the stream should be empty.
	if c.optional[".lst"] {
		compare(t, c, base, ".lst", lst.String())
	}
	if why, unstable := c.unstable[base]; unstable {
		t.Logf("%s: debugging stream not compared: %s\n", base, why)
		return
	}
	compare(t, c, base, ".err", dbg.String())
}

// compare checks one stream against its golden file, or rewrites the golden
// when -update was given for a corpus this repository owns.
func compare(t *testing.T, c corpus, base, ext, got string) {
	t.Helper()
	name := filepath.Join(c.dir, base+ext)

	want, err := os.ReadFile(name)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist) && c.optional[ext]:
		// no golden for this stream means it must stay empty
		if got != "" {
			t.Errorf("%s: want an empty stream: got\n%s\n", base+ext, got)
		}
		return
	case errors.Is(err, fs.ErrNotExist) && *update && c.writable:
		// -update may refresh a golden, but creating one would enshrine
		// whatever the engine does today as the specification
		t.Fatalf("%s: will not create a golden file; write it by hand first\n", name)
	default:
		t.Fatalf("%s: %v\n", name, err)
	}

	if *update && c.writable {
		if ok, _, _, _ := c.equal(got, string(want)); ok {
			return
		}
		if err := os.WriteFile(name, []byte(got), 0644); err != nil {
			t.Fatalf("%s: %v\n", name, err)
		}
		t.Logf("%s: rewritten\n", name)
		return
	}

	ok, line, gotLine, wantLine := c.equal(got, string(want))
	if ok {
		return
	}
	t.Errorf("%s: line %d\n\twant %q\n\t got %q\n", name, line, wantLine, gotLine)
}

// requireEngine skips the whole corpus when the engine has nothing to run.
//
// There is no environment variable and no build tag on purpose: the skip is
// keyed on the sentinel, so it expires by itself the moment the source is in
// place, and nobody has to remember to turn these tests back on. The engine is
// the distributed LOWL source of ML/I, which cannot be committed here, so a
// clone that has not fetched it has no processor to test.
func requireEngine(t *testing.T) {
	t.Helper()
	var out, dbg bytes.Buffer
	_, err := ml1.Run(ml1.Job{
		Inputs:     []ml1.Input{ml1.StringInput("probe.ml1", "")},
		Outputs:    []io.Writer{&out},
		Debug:      &dbg,
		LOWLSource: lowlSource(),
	})
	if errors.Is(err, ml1.ErrNoEngineSource) {
		t.Skipf("%v; run: go run ./cmd/fetchtestdata\n", err)
	}
}

// lowlSource is where the LOWL source of ML/I is unpacked, if it is here at
// all. The path is absolute because the tests run from the package directory
// and the archives are unpacked at the top of the repository.
func lowlSource() string {
	root := moduleRoot()
	for _, candidate := range []string{
		filepath.Join(root, ".downloads", "lowlml1", "ml1ajb.lwl"),
		filepath.Join(root, ".references", "ml1aih.lwl"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// a name that will not resolve, so Run reports the sentinel rather than
	// searching the working directory and finding something unexpected
	return filepath.Join(root, ".downloads", "lowlml1", "ml1ajb.lwl")
}

// isProcessError reports whether err is one of the outcomes a golden file is
// allowed to describe, as opposed to a fault in the test itself.
func isProcessError(err error) bool {
	return errors.Is(err, ml1.ErrAborted) || errors.Is(err, ml1.ErrProcessErrors)
}

// maxInputStreams is the number of input files ML/I accepts, from AA.2:
// "there may be no more than five of these". S10 selects between them.
const maxInputStreams = 5

// extraStreamOf reports which input stream NAME.N.ml1 is, or zero when the
// file name is an ordinary case rather than one of a case's extra streams.
//
// The convention is only a naming one: a case that wants to switch streams
// with S10 needs somewhere to switch *to*, and the corpus is a flat directory
// of NAME.ml1 files, so the stream number goes in the name.
func extraStreamOf(name string) int {
	base := strings.TrimSuffix(name, ".ml1")
	if len(base) < 3 || base[len(base)-2] != '.' {
		return 0
	}
	n := int(base[len(base)-1] - '0')
	if n < 2 || n > maxInputStreams {
		return 0
	}
	return n
}

// extraStreams collects the input streams beyond the case's own, NAME.2.ml1
// through NAME.5.ml1, in order and stopping at the first gap. A case with none
// gets none, which is every case but one.
//
// The numbering only works out because this corpus has no prelude: the case
// itself is stream 1, so NAME.2.ml1 lands on stream 2 where MCSET S10=2 will
// look for it. A corpus with a prelude would be off by one, so say so rather
// than let a future case be quietly misnumbered.
func extraStreams(t *testing.T, c corpus, base string) []ml1.Input {
	t.Helper()
	var inputs []ml1.Input
	for n := 2; n <= maxInputStreams; n++ {
		name := fmt.Sprintf("%s.%d.ml1", base, n)
		data, err := os.ReadFile(filepath.Join(c.dir, name))
		if errors.Is(err, fs.ErrNotExist) {
			break
		} else if err != nil {
			t.Fatalf("%s: %v\n", name, err)
		}
		if c.prelude != "" {
			t.Fatalf("%s: extra input streams and a prelude cannot be combined; the numbering would be off by one\n", name)
		}
		inputs = append(inputs, ml1.BytesInput(name, data))
	}
	return inputs
}

// casesIn lists the cases of a corpus: every NAME.ml1 that is not the
// prelude, and not one of a case's extra input streams, and that has a
// NAME.out beside it. Discovery rather than a hardcoded list, so that an
// archive which gains or loses a test does not silently diverge from what we
// run.
func casesIn(t *testing.T, c corpus) []string {
	t.Helper()
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		t.Fatalf("%s: %v\n", c.name, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ml1") || e.Name() == c.prelude {
			continue
		}
		// an extra input stream belongs to the case it is named after, so it
		// is not a case itself. This is checked before the .out golden is
		// looked for, because a missing golden there would otherwise be
		// reported as an oversight.
		if extraStreamOf(e.Name()) != 0 {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".ml1")
		if _, err := os.Stat(filepath.Join(c.dir, base+".out")); err != nil {
			t.Logf("%s: %s has no .out golden; skipping it\n", c.name, base)
			continue
		}
		names = append(names, base)
	}
	sort.Strings(names)
	return names
}

// moduleRoot walks up from the working directory looking for go.mod, so that
// a path outside this package survives the package being moved.
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
