// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

// Command macl is the L route, end to end.
//
// There are two ways to port ML/I. cmd/maclo takes the first: it runs the LOWL
// source ML/I is distributed as, on the virtual machine in pkg/lowl, from an
// engine built into the binary. macl is the second — L is the language the
// machine-independent logic of ML/I is written in, pkg/l reads it, and
// pkg/l/lmap maps it into LOWL for the same machine to run. So macl reads an
// L program, says what it found, and runs it.
//
//	macl check   ml1aie.l    scan, parse and resolve; report what is wrong
//	macl summary ml1aie.l    count what the front end saw
//	macl list    ml1aie.l    print the statement listing, indented by nesting
//	macl symbols ml1aie.l    print the symbol table
//	macl source  ml1aie.l    print the program back as L
//	macl lowl    ml1aie.l    print the LOWL it maps into
//	macl run     ml1aie.l --source file.ml1    process a macro file with it
//
// It is not cmd/lcheck. lcheck is the tool for working on the front end and
// dumps the stages — the token stream, one line per source line — the way
// `lasm --test-scanner` does for LOWL. macl reports on the program rather than
// on the parse, and it is the one the back end is behind. The relationship is
// the one cmd/lasm and cmd/maclo already have.
//
// Nothing under pkg/l opens a file or touches a process stream, so every
// listing here goes to a path the caller named and a run that asks for none
// leaves the directory it ran in alone.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mdhender/maclo"
	"github.com/mdhender/maclo/pkg/l"
	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/sema"
)

// The exit codes.
//
// The first three are macl's own and say what it made of the L it was given.
// run adds the two ML/I reports its own processes with, which are the ones
// Appendix AA of the user's manual specifies and cmd/maclo returns: 254 when
// the process ran and found errors in the text, 255 when it could not finish.
// They are far apart from macl's on purpose -- "your L is wrong" and "the
// macros in your input are wrong" are different answers.
const (
	exitOK     = 0
	exitErrors = 1 // the source had errors
	exitUsage  = 2 // the command line was wrong, or a file would not open
)

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr))
}

// env is the three streams a command may use, so that main_test.go can drive
// one without touching the process's own.
type env struct {
	stdout io.Writer
	stderr io.Writer
}

func dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	e := &env{stdout: stdout, stderr: stderr}
	name, rest := args[0], args[1:]

	switch name {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	case "version", "--version":
		fmt.Fprintf(stdout, "macl %s\n", maclo.Version())
		return exitOK

	case "check":
		return report(e, name, rest, nil)
	case "summary":
		return report(e, name, rest, func(w io.Writer, r *l.Result) error {
			return l.WriteSummary(w, r.Summary())
		})
	case "list":
		return report(e, name, rest, func(w io.Writer, r *l.Result) error {
			return ast.WriteListing(w, r.Program)
		})
	case "symbols":
		return report(e, name, rest, func(w io.Writer, r *l.Result) error {
			return sema.WriteListing(w, r.Table)
		})
	case "source":
		return report(e, name, rest, func(w io.Writer, r *l.Result) error {
			return ast.WriteSource(w, r.Program)
		})
	case "lowl":
		return lowlProgram(e, rest)
	case "run":
		return runProgram(e, rest)
	}

	fmt.Fprintf(stderr, "macl: %s is not a macl command\n\n", name)
	fmt.Fprint(stderr, usage)
	return exitUsage
}

// opts is what every command that reads an L source accepts.
type opts struct {
	out       string
	maxErrors int
	quiet     bool
	source    string
}

// parse reads a command's flags and the one source file it works on.
//
// listing is false for check, which produces no listing and so has no --out to
// offer. Flags after the file name are accepted as well as before it: the
// standard flag package stops at the first non-flag, and having `macl list
// ml1aie.l --out x` fail would be a surprise nobody needs.
func parse(e *env, name string, args []string, listing bool) (*opts, int, bool) {
	o := &opts{out: "-"}
	fs := flag.NewFlagSet("macl "+name, flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	if listing {
		fs.StringVar(&o.out, "out", "-", `write it here ("-" is the standard output)`)
	}
	fs.IntVar(&o.maxErrors, "max-errors", 20, "stop reporting diagnostics after this many (0 for all)")
	fs.BoolVar(&o.quiet, "quiet", false, "report diagnostics and nothing else")

	rest := args
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			return nil, exitUsage, false // flag has already said what was wrong
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		if o.source != "" {
			fmt.Fprintf(e.stderr, "macl %s: one L source at a time, and %s is a second\n", name, rest[0])
			return nil, exitUsage, false
		}
		o.source, rest = rest[0], rest[1:]
	}
	if o.source == "" {
		fmt.Fprintf(e.stderr, "macl %s: no L source to read\n\nusage: macl %s [options] FILE.l\n", name, name)
		fs.PrintDefaults()
		return nil, exitUsage, false
	}
	return o, exitOK, true
}

// report is the shape every reading command has: read the file, run the front
// end, write whatever this command writes, then say what was wrong with it.
//
// The listing is written even when the source has errors. The stages
// accumulate rather than stopping, so a partial answer exists, and a partial
// answer is what makes a listing useful while a file is still being fixed.
func report(e *env, name string, args []string, write func(io.Writer, *l.Result) error) int {
	o, code, ok := parse(e, name, args, write != nil)
	if !ok {
		return code
	}
	src, err := os.ReadFile(o.source)
	if err != nil {
		fmt.Fprintf(e.stderr, "macl %s: %v\n", name, err)
		return exitUsage
	}

	result := l.Parse(src)
	if write != nil {
		if err := emit(o.out, e.stdout, func(w io.Writer) error { return write(w, result) }); err != nil {
			fmt.Fprintf(e.stderr, "macl %s: %v\n", name, err)
			return exitUsage
		}
	}

	diagnose(e.stderr, o, result)
	if result.HasErrors() {
		return exitErrors
	}
	return exitOK
}

// diagnose writes the diagnostics in source order and then a verdict.
func diagnose(w io.Writer, o *opts, result *l.Result) {
	s := result.Summary()
	var shown int
	for _, d := range result.Errs.Sorted() {
		if o.maxErrors > 0 && shown >= o.maxErrors {
			break
		}
		shown++
		fmt.Fprintf(w, "%s:%s: %s: %s\n", o.source, d.Pos, d.Severity, d.Msg)
	}
	if n := s.Errors + s.Warnings; n > shown {
		fmt.Fprintf(w, "%s: %d more not shown (--max-errors)\n", o.source, n-shown)
	}
	if o.quiet {
		return
	}
	fmt.Fprintf(w, "%s: %d statements, %d errors, %d warnings\n",
		o.source, s.Statements, s.Errors, s.Warnings)
}

// emit runs write against the named destination. "-" is the standard output;
// anything else is a file, and it is the only place this program creates one.
func emit(path string, stdout io.Writer, write func(io.Writer) error) error {
	if path == "-" {
		return write(stdout)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := write(f); err != nil {
		return err
	}
	return f.Close()
}

const usage = `usage: macl <command> [options] FILE.l

Reads L, the machine-independent language the logic of ML/I is written in.

  check      scan, parse and resolve; report what is wrong
  summary    count what the front end saw: statements, sections, names
  list       print the statement listing, indented by nesting
  symbols    print the symbol table
  source     print the program back as L, which is what the round trip checks
  lowl       print the LOWL the program maps into
  run        map the program and process a macro file with it
  version    print the version of this port

Options for the reading commands:

  --out file       write the listing here ("-" is the standard output)
  --max-errors n   stop reporting diagnostics after this many; 0 for all
  --quiet          report diagnostics and nothing else

Options for run:

  --source file    input text to process; give it again for a second stream
  --out file       write the results here ("-" is the standard output)
  --output file    an extra output stream, after the first
  --debug file     where messages go ("-" is the standard error)
  --listing file   write the listing S20 controls here
  --workspace n    words of workspace available to ML/I

    macl run ml1aie2.l --source file.ml1

Exit status is 0 when the source is clean, 1 when it has errors, and 2 when the
command line was wrong or a file would not open. run also returns what ML/I
returns: 254 when the process found errors in the text it was processing, 255
when it could not finish.

The other three ML/I programs in this repository run the LOWL that ML/I is
distributed as rather than translating the L it is written in: cmd/maclo runs
an engine built into the binary, cmd/ml1 follows the operating instructions in
Appendix AA of the user's manual, and cmd/lasm assembles LOWL on its own.
cmd/lcheck is the other L program: it dumps the front end's stages and is the
tool for working on pkg/l rather than on a program written in L.
`
