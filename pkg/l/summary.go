// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package l

import (
	"bufio"
	"fmt"
	"io"
	"sort"

	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/sema"
	"github.com/mdhender/maclo/pkg/l/stmt"
	"github.com/mdhender/maclo/pkg/l/token"
)

// Summary is what a run of the front end saw, counted.
//
// It lives here rather than in cmd/macl because it is an answer about an L
// program and not a way of printing one: a command that computed it for itself
// would be behaviour nothing tests. TestML1AIE asserts every field of it
// against the real 2,510 lines of ML/I, which is also what stops the numbers a
// command prints from being merely plausible.
type Summary struct {
	// Lines is the last line the scanner reached, and Tokens the length of the
	// stream it produced.
	Lines  int
	Tokens int

	// Statements counts every statement in the tree, including the ones a
	// compound statement holds. Closers are not among them: each is folded
	// onto the statement it closes.
	Statements  int
	ByStatement map[stmt.Kind]int

	// Sections names the SECTIONs in the order the file writes them.
	Sections []string

	// MaxIfDepth and MaxChainDepth are how deep the two nested constructs the
	// manual restricts actually go (lmap.txt 4.2.1, 4.2.2). A one-line IF does
	// not count towards the first: it opens nothing.
	MaxIfDepth    int
	MaxChainDepth int

	// Names counts the symbols a listing would show - the declared ones and
	// the undeclared ones, but not the predefined vocabulary nothing referred
	// to. Undeclared is the number of names used and never declared.
	Names      int
	ByName     map[sema.Kind]int
	Undeclared int
	Predefined int
	Errors     int
	Warnings   int
}

// Summary counts the result. It is safe on a result with diagnostics in it:
// the stages accumulate rather than stopping, so the tree and the table are
// there to be counted either way.
func (r *Result) Summary() *Summary {
	s := &Summary{
		Tokens:      len(r.Tokens),
		ByStatement: map[stmt.Kind]int{},
		ByName:      map[sema.Kind]int{},
	}
	// The last token is the EOF, and it sits one line past a source that ends
	// in a newline - which every L source does. The last one before it is on
	// the last line there is.
	for i := len(r.Tokens) - 1; i >= 0; i-- {
		if r.Tokens[i].Kind != token.EOF {
			s.Lines = r.Tokens[i].Pos.Line
			break
		}
	}

	ast.Inspect(r.Program, func(n ast.Node) bool {
		if st, ok := n.(ast.Stmt); ok {
			s.Statements++
			s.ByStatement[st.Kind()]++
		}
		return true
	})
	for _, top := range r.Program.Stmts {
		if sec, ok := top.(*ast.Section); ok && sec.Name != nil {
			s.Sections = append(s.Sections, sec.Name.Text)
		}
	}
	s.MaxIfDepth, s.MaxChainDepth = depths(r.Program.Stmts)

	for _, sym := range r.Table.Symbols() {
		switch {
		case sym.Predefined:
			// The whole vocabulary of L is in the table whether a program
			// mentions it or not, so count the ones it mentioned. That is also
			// what sema.WriteListing shows, and two counts of the same thing
			// disagreeing is worse than either.
			if len(sym.Refs) > 0 {
				s.Predefined++
			}
			continue
		case sym.Kind == sema.Undefined:
			s.Undeclared++
		}
		s.Names++
		s.ByName[sym.Kind]++
	}

	for _, e := range r.Errs {
		if e.Severity == token.SevError {
			s.Errors++
		} else {
			s.Warnings++
		}
	}
	return s
}

// depths measures the two nestings the manual makes claims about. The walk is
// explicit rather than an ast.Inspect because the depth of a construct is a
// property of the path to it and Inspect does not hand one back out.
func depths(list []ast.Stmt) (maxIf, maxChain int) {
	var ifDepth, chainDepth int
	var walk func([]ast.Stmt)
	walk = func(list []ast.Stmt) {
		for _, s := range list {
			switch v := s.(type) {
			case *ast.If:
				if !v.Block {
					walk(v.Stmts()) // a one-line IF opens nothing
					continue
				}
				ifDepth++
				maxIf = max(maxIf, ifDepth)
				walk(v.Body)
				ifDepth--
			case *ast.ChainFrom:
				chainDepth++
				maxChain = max(maxChain, chainDepth)
				walk(v.Body)
				chainDepth--
			case *ast.Section:
				walk(v.Body)
			case *ast.BlockDec:
				walk(v.Body)
			case *ast.Subroutine:
				walk(v.Body)
			case *ast.LinkRoutine:
				walk(v.Body)
			}
		}
	}
	walk(list)
	return maxIf, maxChain
}

// WriteSummary prints a summary. Like every listing in pkg/l it takes an
// io.Writer and names no file.
//
// The two tallies are ordered by count and then by name rather than by the
// enum, because what a reader wants from them is the shape of the program.
// Ordering them at all is the point: a map ranged over directly would give a
// different listing on every run, which is a lesson this package learnt from
// its own golden files.
func WriteSummary(w io.Writer, s *Summary) error {
	bw := bufio.NewWriter(w)

	fmt.Fprintf(bw, "%8d  lines\n", s.Lines)
	fmt.Fprintf(bw, "%8d  tokens\n", s.Tokens)
	fmt.Fprintf(bw, "%8d  statements\n", s.Statements)
	fmt.Fprintf(bw, "%8d  names declared or used\n", s.Names)
	fmt.Fprintf(bw, "%8d  used and never declared\n", s.Undeclared)
	fmt.Fprintf(bw, "%8d  of L's own names referred to\n", s.Predefined)
	fmt.Fprintf(bw, "%8d  errors, %d warnings\n", s.Errors, s.Warnings)
	fmt.Fprintf(bw, "%8d  deepest block IF, %d deepest CHAIN FROM\n", s.MaxIfDepth, s.MaxChainDepth)

	fmt.Fprintf(bw, "\nsections (%d)\n", len(s.Sections))
	for i, name := range s.Sections {
		fmt.Fprintf(bw, "  %2d  %s\n", i+1, name)
	}

	fmt.Fprintf(bw, "\nstatements (%d kinds)\n", len(s.ByStatement))
	for _, e := range tally(s.ByStatement, stmt.Kind.String) {
		fmt.Fprintf(bw, "  %-14s %5d\n", e.name, e.n)
	}

	fmt.Fprintf(bw, "\nnames (%d kinds)\n", len(s.ByName))
	for _, e := range tally(s.ByName, sema.Kind.String) {
		fmt.Fprintf(bw, "  %-14s %5d\n", e.name, e.n)
	}

	return bw.Flush()
}

type tallyEntry struct {
	name string
	n    int
}

// tally turns one of the count maps into a stable order: most first, ties
// broken by name.
func tally[K comparable](counts map[K]int, name func(K) string) []tallyEntry {
	out := make([]tallyEntry, 0, len(counts))
	for k, n := range counts {
		out = append(out, tallyEntry{name(k), n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].n != out[j].n {
			return out[i].n > out[j].n
		}
		return out[i].name < out[j].name
	})
	return out
}
