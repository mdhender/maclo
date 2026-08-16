// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

// Package main implements the ML/I macro processor.
package main

/*
 * command line options are taken from https://www.ml1.org.uk/htmldoc/ml1appaa.html
 */

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mdhender/maclo"
	"github.com/mdhender/maclo/pkg/ml1"
)

const (
	// defaultWorkspace is the amount of workspace, in words, given to ML/I
	// when the -w option is not supplied.
	defaultWorkspace = ml1.DefaultWorkspace

	// defaultDebugWidth is the column at which the debugging file is wrapped
	// when the -c option is not supplied. It matches the reference
	// implementation, whose output the test suite was recorded from.
	defaultDebugWidth = ml1.DefaultDebugWidth

	// maxOutputFiles and maxInputFiles are the limits imposed by the
	// operating instructions.
	maxOutputFiles = ml1.MaxOutputs
	maxInputFiles  = ml1.MaxInputs

	// sourceEnv names the LOWL source of ML/I when -s is not given. The
	// processor is that source rather than a translation of it, and its
	// licence keeps it out of this repository, so where it lives is a
	// property of the machine rather than of the program.
	sourceEnv = ml1.EngineEnv
)

// config holds the options and file names taken from the command line.
type config struct {
	version     bool     // true if -v was given
	help        bool     // true if --help was given
	fetchEngine bool     // true if --fetch-engine was given
	showEngine  bool     // true if --engine was given
	workspace   int      // words of workspace, from -w
	debugWidth  int      // wrap column for the debugging file, from -c; 0 never wraps
	listing     string   // from -l; "" means no listing is produced
	debug       string   // from -d; "" means the standard error stream
	source      string   // from -s; "" falls back to the search in pkg/ml1
	output      []string // from -o; empty means the standard output
	input       []string // everything else; empty means the standard input
}

// go run ./cmd/ml1 -v sets18.ml1 foo.ml1 -o foo.tmp_out -d foo.tmp_err
func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run wires the command line onto the processor and returns the exit status.
//
// The standard streams are parameters so that the whole command can be
// exercised from buffers, and main is left as the only thing that calls
// os.Exit. That split matters: the output files are buffered, and os.Exit
// would skip the flush.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "ml1: %v\n", err)
		_, _ = fmt.Fprint(stderr, usage)
		return 1
	}

	// The three extensions that do something instead of processing text, in
	// the order a first-time user meets them. Each one answers and stops: none
	// of them is part of the operating instructions, so none of them has a
	// defined interaction with an input file, and doing both would have to
	// invent one.
	switch {
	case cfg.help:
		_, _ = fmt.Fprint(stdout, usage)
		return 0
	case cfg.showEngine:
		return showEngine(stdout)
	case cfg.fetchEngine:
		return fetchEngine(stdout, stderr)
	}

	// -v only asks for the version number to be printed; the operating
	// instructions don't make it stop the run, so any input files given
	// alongside it are still processed. The banner goes to the standard
	// error so that it can't be mistaken for macro output when the
	// standard output is being used as an output file.
	if cfg.version {
		_, _ = fmt.Fprintf(stderr, "ml1: version %s\n", maclo.Version())
	}

	// AA.4: all files are opened as soon as ML/I is entered, and a failure to
	// open one ends the process at once.
	//
	// The status for that is 1, not 255. AA reserves 255 for "a fatal error
	// caused ML/I to terminate the process prematurely", and a file that will
	// not open means the process never began — the same class of failure as an
	// unusable command line, which already exits 1 above. The reference
	// implementation agrees: it exits 1 for an input, output, debugging or
	// listing file it cannot open.
	job, closeAll, err := cfg.job(stdin, stdout, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "ml1: %v\n", err)
		return 1
	}

	res, runErr := ml1.Run(job)

	// flush before reporting, so that a diagnostic never appears ahead of the
	// output it refers to
	if err := closeAll(); err != nil {
		_, _ = fmt.Fprintf(stderr, "ml1: %v\n", err)
		return 255
	}

	switch {
	case runErr == nil:
	case errors.Is(runErr, ml1.ErrNoEngineSource):
		_, _ = fmt.Fprintf(stderr, "ml1: %v\n", runErr)
		_, _ = fmt.Fprintf(stderr, "ml1: run `ml1 --fetch-engine` to install it,"+
			" or name one with -s or $%s\n", sourceEnv)
		return 1
	case errors.Is(runErr, ml1.ErrAborted), errors.Is(runErr, ml1.ErrProcessErrors):
		// expected outcomes; the diagnostics are already on the debugging
		// stream and the exit status carries the rest
	default:
		_, _ = fmt.Fprintf(stderr, "ml1: %v\n", runErr)
		return 255
	}
	return res.ExitStatus()
}

// job turns a parsed command line into a job, along with a function that
// flushes and closes everything the job opened.
//
// The name - means the standard input or the standard output, and a name used
// more than once resolves to the same writer, so that -o - -d - interleaves
// the way the shell would rather than through two buffers racing over one
// file descriptor.
func (cfg *config) job(stdin io.Reader, stdout, stderr io.Writer) (ml1.Job, func() error, error) {
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

	// -s names the engine outright. Leaving it empty is not a failure: it hands
	// the question to ml1.EnginePaths, which knows $ML1_LOWL_SOURCE, the
	// per-user directory --fetch-engine writes to, and the layout of a
	// developer's checkout. Resolving it in one place is what keeps the command
	// and a program embedding the library finding the same file.
	job := ml1.Job{Workspace: cfg.workspace, DebugWidth: cfg.debugWidth, LOWLSource: cfg.source}

	// no input file at all means the standard input, and so does the name -
	names := cfg.input
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

	// no output file at all means the standard output
	outs := cfg.output
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

	// the debugging stream defaults to the standard error
	job.Debug = stderr
	if cfg.debug != "" {
		w, err := sink(cfg.debug)
		if err != nil {
			return fail(err)
		}
		job.Debug = w
	}

	// no -l means no listing at all
	if cfg.listing != "" {
		w, err := sink(cfg.listing)
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

const usage = `usage: ml1 [-v] [-w words] [-c columns] [-l file] [-d file] [-o file]... [file]...
  -v       print the version number of this implementation of ML/I
  -w n     set the workspace available to ML/I to n words (default 5000)
  -c n     wrap the debugging file at column n; 0 never wraps (default 72)
           this option is an extension, not part of the operating instructions
  -l file  nominate file as the listing file (default: no listing)
  -d file  nominate file as the debugging file (default: standard error)
  -s file  the LOWL source of ML/I to run; overrides the search below
           this option is an extension, not part of the operating instructions
  -o file  nominate file as an output file, at most 4 (default: standard output)
  file     input file, at most 5 (default: standard input)
  The name - means the standard output for -l, -d, and -o, and the standard
  input for an input file. Upper case option letters are also accepted.

The long options are extensions too. Each one answers and stops:
  --help          print this text
  --engine        report where the LOWL source of ML/I is looked for
  --fetch-engine  download it from ml1.org.uk into the per-user directory

ML/I is distributed as LOWL source and this program runs that source rather
than a translation of it, so the processor is a file that has to be on the
machine. Its licence forbids redistributing a machine readable copy, which is
why it is neither built in nor shipped alongside. It is looked for in
$ML1_LOWL_SOURCE, then in $ML1_HOME or the per-user directory, then in a
developer checkout; --engine prints the list in full.
`

// parseArgs parses the command line arguments the original ML/I accepts.
//
// Options and input files may be interleaved, so the arguments are walked by
// hand instead of using the flag package, which stops at the first argument
// that is not an option. An option's value may be attached to the option
// letter (-w5000) or given as the following argument (-w 5000).
func parseArgs(args []string) (*config, error) {
	cfg := &config{workspace: defaultWorkspace, debugWidth: defaultDebugWidth}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// The long options are all extensions, and all of them do something
		// other than process text. They are spelled with two dashes so that
		// they cannot collide with the single letters of the operating
		// instructions, and taken first so that the letter parser below never
		// sees an argument beginning --.
		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--help":
				cfg.help = true
			case "--engine":
				cfg.showEngine = true
			case "--fetch-engine":
				cfg.fetchEngine = true
			default:
				return nil, fmt.Errorf("unknown option: %s", arg)
			}
			continue
		}

		// a lone - is an input file (the standard input), not an option
		if len(arg) < 2 || !strings.HasPrefix(arg, "-") {
			if len(cfg.input) >= maxInputFiles {
				return nil, fmt.Errorf("too many input files (at most %d)", maxInputFiles)
			}
			cfg.input = append(cfg.input, arg)
			continue
		}

		// upper and lower case option letters are both accepted
		letter := strings.ToLower(arg[1:2])
		value, attached := arg[2:], len(arg) > 2

		// -v is the only option that does not take a value
		if letter == "v" {
			if attached {
				return nil, fmt.Errorf("option -%s does not take a value", letter)
			}
			cfg.version = true
			continue
		}

		switch letter {
		case "c", "d", "l", "o", "s", "w":
			if !attached {
				if i+1 >= len(args) {
					return nil, fmt.Errorf("option -%s requires a value", letter)
				}
				i++
				value = args[i]
			}
		default:
			return nil, fmt.Errorf("unknown option: %s", arg)
		}

		switch letter {
		case "c":
			// zero is meaningful here: it asks for no wrapping at all, which
			// is why this is not folded in with the -w check below
			columns, err := strconv.Atoi(value)
			if err != nil || columns < 0 {
				return nil, fmt.Errorf("option -%s wants a column number, got %q", letter, value)
			}
			cfg.debugWidth = columns
		case "d":
			cfg.debug = value
		case "l":
			cfg.listing = value
		case "s":
			cfg.source = value
		case "o":
			if len(cfg.output) >= maxOutputFiles {
				return nil, fmt.Errorf("too many output files (at most %d)", maxOutputFiles)
			}
			cfg.output = append(cfg.output, value)
		case "w":
			words, err := strconv.Atoi(value)
			if err != nil || words < 1 {
				return nil, fmt.Errorf("option -w wants a positive number of words, got %q", value)
			}
			cfg.workspace = words
		}
	}

	return cfg, nil
}
