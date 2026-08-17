// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mdhender/maclo/pkg/ml1"
)

// runProgram is the verb this command is named for.
//
// It runs the whole route in one go: the L source is scanned, parsed and
// resolved, mapped into LOWL, assembled, and then run over the input text the
// caller named. A macro file processed by an ML/I that was translated out of L
// on the way in.
//
// The front end runs first and its diagnostics come out first, because a
// program that does not resolve has nothing to run and the reason is worth
// having on its own. Only then does anything of ML/I's start.
func runProgram(e *env, args []string) int {
	o, code, ok := parseRun(e, "run", args)
	if !ok {
		return code
	}
	if len(o.sources) == 0 {
		fmt.Fprintf(e.stderr, "macl run: no input text to process; name one with --source\n")
		return exitUsage
	}

	job, cleanup, err := o.job(e)
	if err != nil {
		fmt.Fprintf(e.stderr, "macl run: %v\n", err)
		return exitUsage
	}
	res, runErr := ml1.Run(job)
	if err := cleanup(); err != nil {
		fmt.Fprintf(e.stderr, "macl run: %v\n", err)
		return exitUsage
	}
	switch {
	case runErr == nil:
	case errors.Is(runErr, ml1.ErrLSourceErrors):
		// the L source is what is wrong, and every reason is in the error
		reportError(e.stderr, "run", runErr)
		return exitErrors
	case errors.Is(runErr, ml1.ErrAborted), errors.Is(runErr, ml1.ErrProcessErrors):
		// the process said what was wrong on its own debugging stream, and the
		// exit status carries the rest
	default:
		fmt.Fprintf(e.stderr, "macl run: %v\n", runErr)
		return res.ExitStatus()
	}
	return res.ExitStatus()
}

// lowlProgram writes the LOWL an L program maps into.
//
// It is the other half of run, and it is here rather than beside the listings
// because it reports on a program the way run does: the answer is the engine,
// and looking at it is how you find out why a run did what it did. Feeding it
// to lasm is the other reason -- that is the only way to see the assembler's
// own listing of a program that came from L.
func lowlProgram(e *env, args []string) int {
	o, code, ok := parseRun(e, "lowl", args)
	if !ok {
		return code
	}
	engine, code, ok := translate(e, "lowl", o)
	if !ok {
		return code
	}
	if err := emit(o.out, e.stdout, func(w io.Writer) error {
		_, err := w.Write(engine)
		return err
	}); err != nil {
		fmt.Fprintf(e.stderr, "macl lowl: %v\n", err)
		return exitUsage
	}
	return exitOK
}

// translate reads the L source and maps it, reporting whichever half failed.
func translate(e *env, name string, o *runOpts) ([]byte, int, bool) {
	src, err := os.ReadFile(o.program)
	if err != nil {
		fmt.Fprintf(e.stderr, "macl %s: %v\n", name, err)
		return nil, exitUsage, false
	}
	engine, err := ml1.MapL(src)
	if err != nil {
		reportError(e.stderr, name, err)
		return nil, exitErrors, false
	}
	return engine, exitOK, true
}

// report writes a multi-line error one line at a time, each named.
//
// The front end accumulates, so an error from it is every reason at once
// rather than the first one, and a reason with the command in front of it is
// one a reader can scan a column of.
func reportError(w io.Writer, name string, err error) {
	for _, line := range strings.Split(strings.TrimRight(err.Error(), "\n"), "\n") {
		fmt.Fprintf(w, "macl %s: %s\n", name, line)
	}
}

// runOpts is what run and lowl accept. It is a flag set of its own rather than
// the one the listing commands share, because the two have nothing in common
// past the file name: run has streams and a workspace and no --max-errors,
// because a program that does not resolve is not run at all.
type runOpts struct {
	program   string
	sources   stringList
	outputs   stringList
	out       string
	debug     string
	listing   string
	workspace int
}

// stringList collects a flag that may be given more than once.
type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func parseRun(e *env, name string, args []string) (*runOpts, int, bool) {
	o := &runOpts{out: "-", debug: "-", workspace: ml1.DefaultWorkspace}
	fs := flag.NewFlagSet("macl "+name, flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	if name == "run" {
		fs.Var(&o.sources, "source", "input text to process; give it again for a second stream")
		fs.Var(&o.outputs, "output", "an extra output stream, after the first")
		fs.StringVar(&o.debug, "debug", "-", `where messages go ("-" is the standard error)`)
		fs.StringVar(&o.listing, "listing", "", "write the listing S20 controls here")
		fs.IntVar(&o.workspace, "workspace", ml1.DefaultWorkspace, "words of workspace available to ML/I")
	}
	fs.StringVar(&o.out, "out", "-", `write the results here ("-" is the standard output)`)

	rest := args
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			return nil, exitUsage, false // flag has already said what was wrong
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		if o.program != "" {
			fmt.Fprintf(e.stderr, "macl %s: one L source at a time, and %s is a second\n", name, rest[0])
			return nil, exitUsage, false
		}
		o.program, rest = rest[0], rest[1:]
	}
	if o.program == "" {
		fmt.Fprintf(e.stderr, "macl %s: no L source to read\n\nusage: macl %s [options] FILE.l\n", name, name)
		fs.PrintDefaults()
		return nil, exitUsage, false
	}
	return o, exitOK, true
}

// job builds the ML/I job and the closer that flushes it.
//
// Every file this opens is one the caller named, which is the rule the whole
// program is held to: a run that asks for no files leaves the directory it ran
// in alone.
func (o *runOpts) job(e *env) (ml1.Job, func() error, error) {
	var closers []io.Closer
	cleanup := func() error {
		var first error
		for i := len(closers) - 1; i >= 0; i-- {
			if err := closers[i].Close(); err != nil && first == nil {
				first = err
			}
		}
		return first
	}
	fail := func(err error) (ml1.Job, func() error, error) {
		_ = cleanup()
		return ml1.Job{}, func() error { return nil }, err
	}
	create := func(path string, std io.Writer) (io.Writer, error) {
		if path == "-" {
			return std, nil
		}
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		closers = append(closers, f)
		return f, nil
	}

	job := ml1.Job{Workspace: o.workspace, DebugWidth: ml1.DefaultDebugWidth}
	for _, name := range o.sources {
		data, err := os.ReadFile(name)
		if err != nil {
			return fail(err)
		}
		job.Inputs = append(job.Inputs, ml1.BytesInput(name, data))
	}
	first, err := create(o.out, e.stdout)
	if err != nil {
		return fail(err)
	}
	job.Outputs = append(job.Outputs, first)
	for _, name := range o.outputs {
		w, err := create(name, e.stdout)
		if err != nil {
			return fail(err)
		}
		job.Outputs = append(job.Outputs, w)
	}
	if job.Debug, err = create(o.debug, e.stderr); err != nil {
		return fail(err)
	}
	if o.listing != "" {
		if job.Listing, err = create(o.listing, e.stdout); err != nil {
			return fail(err)
		}
	}

	// Naming the L source is what asks for the L back end. The engine it maps
	// into never becomes a file: it is a derivative of a source that may be
	// built into a program and not redistributed, so the only way to get one
	// on disk is to ask for it by name with macl lowl.
	job.LSource = o.program
	return job, cleanup, nil
}
