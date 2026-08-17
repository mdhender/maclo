// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

// Package main implements a front end for L: it scans, parses and resolves an
// L source and reports what it finds.
//
// It does not compile. cmd/lasm assembles LOWL and produces a machine to run;
// this is the tool for working on the front end, so it checks and lists and
// stops there; the back end is behind cmd/macl.
//
//	lcheck --source ml1aie.l                     report diagnostics only
//	lcheck --source ml1aie.l --listing -         and print the statement listing
//	lcheck --source ml1aie.l --symbols sym.txt   write the symbol table
//
// Every listing goes to a path the caller names, and "-" means the standard
// output. Nothing under pkg/l opens a file, so unlike cmd/lasm this program
// leaves no artifacts in whatever directory it was run from.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mdhender/maclo/pkg/l"
	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/cst"
	"github.com/mdhender/maclo/pkg/l/scanner"
	"github.com/mdhender/maclo/pkg/l/sema"
	"github.com/mdhender/maclo/pkg/l/token"
)

// config is what the command was asked to do. Plain flags, like cmd/maclo:
// the environment and JSON-file machinery in cmd/lasm/config.go is there for
// historical reasons and buys nothing here.
type config struct {
	source    string
	listing   string
	symbols   string
	tokens    string
	tree      string
	maxErrors int
	quiet     bool
}

func main() {
	cfg := &config{}
	flag.StringVar(&cfg.source, "source", "", "the L source to read (required)")
	flag.StringVar(&cfg.listing, "listing", "", `write the statement listing here ("-" is stdout)`)
	flag.StringVar(&cfg.symbols, "symbols", "", `write the symbol table here ("-" is stdout)`)
	flag.StringVar(&cfg.tokens, "tokens", "", `write the token stream here ("-" is stdout)`)
	flag.StringVar(&cfg.tree, "cst", "", `write one line per source line here ("-" is stdout)`)
	flag.IntVar(&cfg.maxErrors, "max-errors", 20, "stop reporting after this many (0 for all)")
	flag.BoolVar(&cfg.quiet, "quiet", false, "report diagnostics and nothing else")
	flag.Parse()

	code, err := run(cfg, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lcheck: %v\n", err)
		os.Exit(2)
	}
	os.Exit(code)
}

// run does the work and returns the exit code. It takes its streams so that
// main_test.go can drive it without touching the process's own.
func run(cfg *config, stdout, stderr io.Writer) (int, error) {
	if cfg.source == "" {
		return 0, fmt.Errorf("no --source")
	}
	src, err := os.ReadFile(cfg.source)
	if err != nil {
		return 0, err
	}

	result := l.Parse(src)

	if err := emit(cfg.tokens, stdout, func(w io.Writer) error {
		return scanner.WriteTokens(w, result.Tokens)
	}); err != nil {
		return 0, err
	}
	if err := emit(cfg.tree, stdout, func(w io.Writer) error {
		return cst.WriteListing(w, result.File)
	}); err != nil {
		return 0, err
	}
	if err := emit(cfg.listing, stdout, func(w io.Writer) error {
		return ast.WriteListing(w, result.Program)
	}); err != nil {
		return 0, err
	}
	if err := emit(cfg.symbols, stdout, func(w io.Writer) error {
		return sema.WriteListing(w, result.Table)
	}); err != nil {
		return 0, err
	}

	report(cfg, stderr, result)
	if result.HasErrors() {
		return 1, nil
	}
	return 0, nil
}

// report writes the diagnostics in source order.
func report(cfg *config, w io.Writer, result *l.Result) {
	var errs, warns, shown int
	for _, e := range result.Errs.Sorted() {
		if e.Severity == token.SevError {
			errs++
		} else {
			warns++
		}
		if cfg.maxErrors > 0 && shown >= cfg.maxErrors {
			continue
		}
		shown++
		fmt.Fprintf(w, "%s:%s: %s: %s\n", cfg.source, e.Pos, e.Severity, e.Msg)
	}
	if n := errs + warns; n > shown {
		fmt.Fprintf(w, "%s: %d more not shown (--max-errors)\n", cfg.source, n-shown)
	}
	if cfg.quiet {
		return
	}
	fmt.Fprintf(w, "%s: %d statements, %d errors, %d warnings\n",
		cfg.source, countStmts(result.Program), errs, warns)
}

func countStmts(p *ast.Program) int {
	var n int
	ast.Inspect(p, func(node ast.Node) bool {
		if _, ok := node.(ast.Stmt); ok {
			n++
		}
		return true
	})
	return n
}

// emit runs write against the named destination. An empty path means the
// caller did not ask for this listing.
func emit(path string, stdout io.Writer, write func(io.Writer) error) error {
	switch path {
	case "":
		return nil
	case "-":
		return write(stdout)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := write(f); err != nil {
		return err
	}
	return f.Close()
}
