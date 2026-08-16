// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/ml1"
)

// The examples in testdata are a differential suite: there is no golden file,
// and the expectation is whatever the oracle produces from the same input. That
// makes them the widest statement about the engine this repository can make,
// because they are real programs rather than cases written to exercise one
// construct, and because nobody had to decide in advance what the right answer
// was.
//
// They come from Rosetta Code and cannot be committed, so they are gitignored
// and this test skips without them, the way TestGoldenUpstream skips without
// its corpus. testdata/run-examples.sh runs the same comparison against the
// built binary; this runs it against ml1.Run, in process, which is what the
// rest of the suite exercises and what a change to the engine actually
// touches.
//
// Both sides are given the defaults cmd/ml1 uses, since that is the
// configuration the oracle is being compared under: DefaultWorkspace, and a
// debugging stream wrapped at DefaultDebugWidth.

// examplesDir holds the example programs, relative to the module root.
const examplesDir = "testdata"

// example is one program, the extra input stream it reads if it has one, and
// the reason it is expected to disagree with the oracle if it is.
type example struct {
	name string // NAME, for NAME.ml1 and the optional NAME.expected
	data string // a second input stream, in the same directory; "" for none
	skew string // why this one must differ; "" means it must agree
}

// examples is the whole suite, in the order run-examples.sh runs it. It is a
// list rather than a directory walk so that an example's extra input stream is
// known, and so that adding a program without wiring it up is caught below
// rather than passing unnoticed.
var examples = []example{
	{name: "100-doors"},
	{name: "99-bottles-iterative"},
	{name: "99-bottles-recursive"},
	{name: "99-bottles-99-lines"},
	{name: "ackermann"},
	{name: "factorial-iterative"},
	{name: "factorial-recursive"},
	{name: "fibonacci"},
	{name: "literals-string"},
	{name: "special-variables"},
	{name: "hello-stderr"},
	{name: "newline-omission"},
	{name: "a-plus-b", data: "pair.dat"},
	{name: "integer-comparison", data: "pair.dat"},
	{name: "align-columns", data: "cols.dat"},

	// The two below are the minimal reproductions of the version skew, and
	// they are here to be watched rather than to pass: the LOWL source we run
	// is AJB (1986) and the oracle is CKQ, so each of these uses a construct
	// the engine has never heard of. Agreement would mean the feature had
	// arrived, which is a result worth being told about.
	{name: "bitwise-operations", data: "pair.dat",
		skew: `& and | in a macro expression arrived after AJB; GETEXP takes + - * / only`},
	{name: "csv-to-html", data: "csv.dat",
		skew: `MCCVAR does not appear in ml1ajb.lwl at all`},
}

func TestExamplesAgainstOracle(t *testing.T) {
	dir := filepath.Join(moduleRoot(), examplesDir)
	oracle := filepath.Join(moduleRoot(), oraclePath)

	if _, err := os.Stat(oracle); errors.Is(err, fs.ErrNotExist) {
		t.Skipf("no reference implementation at %s; this suite has no golden files and needs one\n", oraclePath)
	} else if err != nil {
		t.Fatalf("%s: %v\n", oraclePath, err)
	}
	// The examples are third party text under CC BY-SA, so they are gitignored
	// and no tool fetches them; a checkout that has not been given them has
	// nothing to run here.
	if present(dir, examples) == 0 {
		t.Skipf("no examples in %s; see its README.md for where they come from\n", examplesDir)
	}
	requireEngine(t)

	checkEveryExampleIsListed(t, dir)

	for _, ex := range examples {
		ex := ex // go 1.20: the loop variable is shared
		t.Run(ex.name, func(t *testing.T) {
			runExample(t, dir, oracle, ex)
		})
	}
}

func runExample(t *testing.T, dir, oracle string, ex example) {
	t.Helper()

	inputs := []string{filepath.Join(dir, ex.name+".ml1")}
	if ex.data != "" {
		inputs = append(inputs, filepath.Join(dir, ex.data))
	}
	for _, name := range inputs {
		if _, err := os.Stat(name); errors.Is(err, fs.ErrNotExist) {
			t.Skipf("%s is not here; see %s/README.md\n", name, examplesDir)
		} else if err != nil {
			t.Fatalf("%s: %v\n", name, err)
		}
	}

	wantOut, wantDbg := runOracle(t, oracle, inputs)
	gotOut, gotDbg := runEngine(t, inputs)

	agrees := true
	for _, s := range []struct {
		stream    string
		got, want string
	}{
		{"the output stream", gotOut, wantOut},
		{"the debugging stream", gotDbg, wantDbg},
	} {
		ok, line, gotLine, wantLine := equalExact(s.got, s.want)
		if ok {
			continue
		}
		agrees = false
		if ex.skew == "" {
			t.Errorf("%s: %s: line %d\n\twant %q\n\t got %q\n", ex.name, s.stream, line, wantLine, gotLine)
		}
	}

	// A few of the tasks publish their expected output on the page. Those are
	// saved beside the program and checked as well, which makes them a third
	// opinion, independent of both engines.
	if published, err := os.ReadFile(filepath.Join(dir, ex.name+".expected")); err == nil {
		ok, line, gotLine, wantLine := equalExact(gotOut, string(published))
		if !ok {
			agrees = false
			if ex.skew == "" {
				t.Errorf("%s: the published output: line %d\n\twant %q\n\t got %q\n",
					ex.name, line, wantLine, gotLine)
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s.expected: %v\n", ex.name, err)
	}

	switch {
	case ex.skew == "":
	case !agrees:
		t.Logf("%s: differs from the oracle, as expected: %s\n", ex.name, ex.skew)
	default:
		t.Errorf("%s: agrees with the oracle, which it is not supposed to: %s\n"+
			"\tthe engine has gained the construct, or the example has stopped using it;\n"+
			"\tdrop the skew note here and in docs/explanation/running-ml1-on-the-lowl-vm.md\n",
			ex.name, ex.skew)
	}
}

// runOracle runs the reference implementation over the same files and returns
// its two streams.
//
// A non-zero exit status is not reported: an example that raises errors ends
// with one, and what is being compared is the text either engine wrote, not
// how it felt about it.
func runOracle(t *testing.T, oracle string, inputs []string) (out, dbg string) {
	t.Helper()
	work := t.TempDir()
	outFile := filepath.Join(work, "out")
	dbgFile := filepath.Join(work, "err")

	args := append(append([]string{}, inputs...), "-o", outFile, "-d", dbgFile)
	cmd := exec.Command(oracle, args...)
	cmd.Stdin = strings.NewReader("")
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("%s: %v\n", oraclePath, err)
		}
	}
	return readAll(t, outFile), readAll(t, dbgFile)
}

// runEngine runs this port over the same files, in process and into buffers,
// with the defaults cmd/ml1 would have given it.
func runEngine(t *testing.T, inputs []string) (out, dbg string) {
	t.Helper()
	var outBuf, dbgBuf bytes.Buffer

	job := ml1.Job{
		Outputs:    []io.Writer{&outBuf},
		Debug:      &dbgBuf,
		Workspace:  ml1.DefaultWorkspace,
		DebugWidth: ml1.DefaultDebugWidth,
		LOWLSource: lowlSource(),
	}
	for _, name := range inputs {
		// FileInput rather than the bytes, because it reopens the file on
		// demand and that is what a rewind of the stream needs
		job.Inputs = append(job.Inputs, ml1.FileInput(name))
	}

	if _, err := ml1.Run(job); err != nil && !isProcessError(err) {
		t.Fatalf("run: %v\n", err)
	}
	return outBuf.String(), dbgBuf.String()
}

func readAll(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("%s: %v\n", name, err)
	}
	return string(data)
}

// present counts how many of the examples are actually on disk, so that an
// empty directory skips instead of reporting seventeen skipped subtests.
func present(dir string, examples []example) int {
	n := 0
	for _, ex := range examples {
		if _, err := os.Stat(filepath.Join(dir, ex.name+".ml1")); err == nil {
			n++
		}
	}
	return n
}

// checkEveryExampleIsListed reports any NAME.ml1 in the directory that the
// table above does not name.
//
// The whole point of this test is that these programs stopped being a suite
// nothing runs. An example dropped into the directory and never added here
// would put one back, silently, so say so. The fix is one line in the table,
// or a row in the README's "sampled but not kept" list and no file.
func checkEveryExampleIsListed(t *testing.T, dir string) {
	t.Helper()
	listed := make(map[string]bool, len(examples))
	for _, ex := range examples {
		listed[ex.name] = true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("%s: %v\n", examplesDir, err)
	}
	var unlisted []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ml1") {
			continue
		}
		if base := strings.TrimSuffix(e.Name(), ".ml1"); !listed[base] {
			unlisted = append(unlisted, base)
		}
	}
	if len(unlisted) == 0 {
		return
	}
	sort.Strings(unlisted)
	t.Errorf("%s: nothing runs %s; add it to the examples table, with the extra input stream it reads\n",
		examplesDir, strings.Join(unlisted, ", "))
}
