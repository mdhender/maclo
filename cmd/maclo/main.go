// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

// Command maclo runs ML/I from an engine built into the binary.
//
// It is the second of two front ends and exists because they answer to
// different masters. cmd/ml1 implements the operating instructions in Appendix
// AA of the ML/I user's manual — single letter options, options and input files
// interleaved, the engine found on disk — and is not free to be improved,
// because being a drop-in for the reference implementation is the whole of what
// it is for. maclo has no such obligation. It takes ordinary Go flags, names
// its options in full, and runs a LOWL source compiled into it rather than one
// it has to go looking for.
//
// The engine is embedded under the terms the source is published on: building
// it into a program is permitted, redistributing the source or the program is
// not. So a binary's engines are a property of the machine that built it, a
// clone of the repository has none, and `go install` produces none either.
// maclo says so rather than failing obscurely — --engines lists what is there,
// and a run with nothing to run explains itself.
//
// Usage:
//
//	maclo [options] [file...]
//	maclo --engines
package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mdhender/maclo"
	"github.com/mdhender/maclo/pkg/ml1"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

type options struct {
	engine    string
	list      bool
	version   bool
	workspace int
	wrap      int
	listing   string
	debug     string
	outputs   stringList
	inputs    []string
}

// stringList collects a flag that may be given more than once.
type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opt, code, done := parse(args, stdout, stderr)
	if done {
		return code
	}

	if opt.version {
		_, _ = fmt.Fprintf(stdout, "maclo %s\n", maclo.Version())
	}
	if opt.list {
		return listEngines(stdout, stderr)
	}

	job, closeAll, err := opt.job(stdin, stdout, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "maclo: %v\n", err)
		return 255
	}

	res, runErr := ml1.Run(job)

	// flush before reporting, so a diagnostic never appears ahead of the
	// output it refers to
	if err := closeAll(); err != nil {
		_, _ = fmt.Fprintf(stderr, "maclo: %v\n", err)
		return 255
	}

	switch {
	case runErr == nil:
	case errors.Is(runErr, ml1.ErrAborted), errors.Is(runErr, ml1.ErrProcessErrors):
		// expected outcomes; the diagnostics are already on the debugging
		// stream and the exit status carries the rest
	default:
		_, _ = fmt.Fprintf(stderr, "maclo: %v\n", runErr)
		return 255
	}
	return res.ExitStatus()
}

// parse reads the command line. The third result is true when the command is
// over — a usage error, or -h — and the caller should return the status.
func parse(args []string, stdout, stderr io.Writer) (*options, int, bool) {
	opt := &options{}
	fs := flag.NewFlagSet("maclo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opt.engine, "engine", "", "engine to run: an embedded name, or the path of a .lwl file")
	fs.BoolVar(&opt.list, "engines", false, "list the engines built into this binary and exit")
	fs.BoolVar(&opt.version, "version", false, "print the version of this port")
	fs.IntVar(&opt.workspace, "workspace", ml1.DefaultWorkspace, "words of workspace available to ML/I")
	fs.IntVar(&opt.wrap, "wrap", ml1.NeverWrap, "column at which to wrap the debugging stream; 0 never wraps")
	fs.StringVar(&opt.listing, "listing", "", "write the listing here, under the control of S20")
	fs.StringVar(&opt.debug, "debug", "", "write the debugging stream here (default: standard error)")
	fs.Var(&opt.outputs, "out", "write an output stream here; repeat for more, at most 4")
	fs.Usage = func() { _, _ = fmt.Fprint(stderr, usage) }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = fmt.Fprint(stdout, usage)
			return nil, 0, true
		}
		return nil, 2, true
	}
	opt.inputs = fs.Args()
	return opt, 0, false
}

// listEngines reports what this binary can run. It is the answer to the only
// question about a maclo binary that cannot be worked out by reading the
// source, since what is in it depends on the machine it was built on.
func listEngines(stdout, stderr io.Writer) int {
	engines := ml1.Engines()
	if len(engines) == 0 {
		_, _ = fmt.Fprint(stderr, noEngines)
		return 1
	}
	for i, e := range engines {
		mark := ""
		if i == 0 {
			mark = "  (default)"
		}
		version := e.Version
		if version == "" {
			version = "-"
		}
		_, _ = fmt.Fprintf(stdout, "%-12s %-4s %7d bytes%s\n", e.Name, version, e.Size, mark)
	}
	return 0
}

// job turns the options into a job, along with a function that flushes and
// closes everything it opened.
func (opt *options) job(stdin io.Reader, stdout, stderr io.Writer) (ml1.Job, func() error, error) {
	var files []*os.File
	var buffers []*bufio.Writer
	sinks := map[string]io.Writer{"-": stdout}

	closeAll := func() error {
		var first error
		for _, b := range buffers {
			if err := b.Flush(); err != nil && first == nil {
				first = err
			}
		}
		for _, f := range files {
			if err := f.Close(); err != nil && first == nil {
				first = err
			}
		}
		return first
	}
	fail := func(err error) (ml1.Job, func() error, error) {
		_ = closeAll()
		return ml1.Job{}, nil, err
	}
	sink := func(name string) (io.Writer, error) {
		if w, ok := sinks[name]; ok {
			return w, nil
		}
		f, err := os.Create(name)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
		b := bufio.NewWriter(f)
		buffers = append(buffers, b)
		sinks[name] = b
		return b, nil
	}

	job := ml1.Job{Workspace: opt.workspace, DebugWidth: opt.wrap}
	if err := opt.chooseEngine(&job); err != nil {
		return fail(err)
	}

	names := opt.inputs
	if len(names) == 0 {
		names = []string{"-"}
	}
	for _, name := range names {
		if name == "-" {
			job.Inputs = append(job.Inputs, stdinInput(stdin))
			continue
		}
		// opened and closed straight away so that a missing file is reported
		// before any output file is truncated; ml1.FileInput reopens it on
		// demand, which is what a rewind needs
		f, err := os.Open(name)
		if err != nil {
			return fail(err)
		}
		_ = f.Close()
		job.Inputs = append(job.Inputs, ml1.FileInput(name))
	}

	outs := opt.outputs
	if len(outs) == 0 {
		outs = []string{"-"}
	}
	for _, name := range outs {
		w, err := sink(name)
		if err != nil {
			return fail(err)
		}
		job.Outputs = append(job.Outputs, w)
	}

	job.Debug = stderr
	if opt.debug != "" {
		w, err := sink(opt.debug)
		if err != nil {
			return fail(err)
		}
		job.Debug = w
	}
	if opt.listing != "" {
		w, err := sink(opt.listing)
		if err != nil {
			return fail(err)
		}
		job.Listing = w
	}

	if err := job.Validate(); err != nil {
		return fail(err)
	}
	return job, closeAll, nil
}

// chooseEngine decides what to run.
//
// A name that this binary was built with wins. Anything else is taken as the
// path of a .lwl file, which is how a source the binary does not carry gets
// run without rebuilding. With no name at all the newest embedded engine runs,
// and a binary with none says so instead of falling back to a search: maclo's
// contract is that it runs what it was built with, and a build with nothing in
// it is a mistake worth reporting rather than papering over.
func (opt *options) chooseEngine(job *ml1.Job) error {
	switch {
	case opt.engine == "":
		if job.Engine = ml1.DefaultEngine(); job.Engine == "" {
			return errors.New(strings.TrimSuffix(noEngines, "\n"))
		}
	case ml1.HasEngine(opt.engine):
		job.Engine = opt.engine
	default:
		// Not a name this binary carries, so it is taken as a path. If there
		// is no such file either, the useful thing to say is what the binary
		// does have: a user who asked for a version by name has mistyped it or
		// built without it, and "no such file" sends them looking for a file
		// they never meant to name.
		if _, err := os.Stat(opt.engine); err != nil {
			return fmt.Errorf("%s is neither an engine built into this binary nor a file\n%s",
				opt.engine, embeddedList())
		}
		job.LOWLSource = opt.engine
	}
	return nil
}

// embeddedList renders what this binary carries, for a diagnostic.
func embeddedList() string {
	engines := ml1.Engines()
	if len(engines) == 0 {
		return "this binary has no engines built into it; run `maclo --engines`"
	}
	names := make([]string, len(engines))
	for i, e := range engines {
		names[i] = e.Name
	}
	return "built in: " + strings.Join(names, ", ")
}

// stdinInput returns an input stream that reads r the first time it is opened
// and replays it after that, so that a macro can rewind the standard input
// even though the underlying reader cannot seek.
func stdinInput(r io.Reader) ml1.Input {
	var data []byte
	var read bool
	return ml1.Input{
		Name: "-",
		Open: func() (io.ReadCloser, error) {
			if !read {
				b, err := io.ReadAll(r)
				if err != nil {
					return nil, err
				}
				data, read = b, true
			}
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
}

const noEngines = `no ML/I engine is built into this binary

ML/I is distributed as LOWL source, and its licence permits building that source
into a program but not redistributing the source or the program. So the engine
cannot be committed and cannot be shipped: it has to be fetched on the machine
that builds, and this binary was built without one.

To build one that has an engine:

    go run ./cmd/fetchtestdata     # writes pkg/ml1/engines/ml1ajb.lwl
    go build ./cmd/maclo

To run a source this binary does not carry, name its path:

    maclo --engine /path/to/ml1ajb.lwl file.ml1
`

const usage = `usage: maclo [options] [file...]
       maclo --engines

Runs ML/I on an engine built into this binary. With no input file it reads the
standard input, and with no --out it writes the standard output.

  --engine name    engine to run: a name this binary was built with, or the
                   path of a .lwl file. Default: the newest embedded engine
  --engines        list the engines built into this binary and exit
  --out file       write an output stream here; repeat for more, at most 4
  --debug file     write the debugging stream here (default: standard error)
  --listing file   write the listing here, under the control of S20
  --workspace n    words of workspace available to ML/I (default 5000)
  --wrap n         wrap the debugging stream at column n; 0 never wraps
  --version        print the version of this port
  -h               print this text

The name - means the standard stream for --out, --debug and an input file.

cmd/ml1 is the other front end: it follows Appendix AA of the ML/I user's manual
option for option and finds its engine on disk. Use it when compatibility with
the reference implementation is what matters, and this when it is not.
`
