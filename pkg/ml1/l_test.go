// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ml1_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/mdhender/maclo/pkg/ml1"
)

// The acceptance test for the L backend.
//
// Everything else about the L route asks whether the translation looks right.
// This asks whether the processor it produces behaves right, which is the only
// question the work was for: the same inputs, run by an engine translated out
// of L and by an engine somebody else translated out of the same L, and both
// streams the same to the byte.
//
// It is compared against the published LOWL rather than against the golden
// files in testdata/local, and that is deliberate. Those were recorded against
// AJB, which is two versions later than the L source: comparing to them would
// report the wording ML/I changed between releases as a fault of this
// translation. AIG is the version nearest the L source, so what is left when
// the two are put side by side is the translation and nothing else.
const lBackendEngine = "ml1aig"

func TestLBackendMatchesAIG(t *testing.T) {
	source := filepath.Join(moduleRoot(), ".downloads", "lml1", "ml1aie2.l")
	if _, err := os.Stat(source); errors.Is(err, fs.ErrNotExist) {
		t.Skipf("%s: not present; run: go test ./pkg/l/lmap -run TestMapML1AIE -v, which says how to make it", source)
	} else if err != nil {
		t.Fatal(err)
	}
	requireEmbedded(t)
	if !hasEngine(lBackendEngine) {
		t.Skipf("%s is not built into this binary; run: go run ./cmd/fetchtestdata", lBackendEngine)
	}

	c := localCorpus()
	cases := casesIn(t, c)
	if len(cases) == 0 {
		t.Fatalf("no cases found in %s", c.dir)
	}

	for _, base := range cases {
		base := base
		t.Run(base, func(t *testing.T) {
			mineOut, mineErr := runWith(t, c, base, ml1.Job{LSource: source})
			theirsOut, theirsErr := runWith(t, c, base, ml1.Job{Engine: lBackendEngine})
			differ(t, base, ".out", mineOut, theirsOut)
			differ(t, base, ".err", mineErr, theirsErr)
		})
	}
}

// runWith runs one case on the engine the job names and returns both streams.
//
// It is the corpus runner's job without the golden files: the inputs and the
// stream settings have to be the two engines' as well, because a difference in
// how the case was run would show up as a difference between the engines.
func runWith(t *testing.T, c corpus, base string, job ml1.Job) (out, dbg string) {
	t.Helper()
	source, err := os.ReadFile(filepath.Join(c.dir, base+".ml1"))
	if err != nil {
		t.Fatalf("%s: %v", base, err)
	}

	inputs := []ml1.Input{ml1.BytesInput(base+".ml1", source)}
	inputs = append(inputs, extraStreams(t, c, base)...)

	var results, messages, listing bytes.Buffer
	job.Inputs = inputs
	job.Outputs = []io.Writer{&results}
	job.Debug = &messages
	job.Listing = &listing
	job.Workspace = ml1.DefaultWorkspace
	job.DebugWidth = c.debugWidth

	if _, err := ml1.Run(job); err != nil && !isProcessError(err) {
		t.Fatalf("%s: run: %v", base, err)
	}
	return results.String(), messages.String()
}

// differ reports the first line the two engines disagree on.
func differ(t *testing.T, base, ext, mine, theirs string) {
	t.Helper()
	ok, line, gotLine, wantLine := equalExact(mine, theirs)
	if ok {
		return
	}
	t.Errorf("%s%s: line %d\n\t%s says %q\n\tthe L backend says %q", base, ext, line, lBackendEngine, wantLine, gotLine)
}

// hasEngine reports whether an engine of that name was built in.
func hasEngine(name string) bool {
	for _, e := range ml1.Engines() {
		if e.Name == name {
			return true
		}
	}
	return false
}
