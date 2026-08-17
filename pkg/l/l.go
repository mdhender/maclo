// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

// Package l is a front end for L, the machine-independent language the logic
// of ML/I is written in.
//
// There are two ways to port ML/I. One is to implement LOWL, the low level
// language it is distributed as, and run the distributed source unchanged;
// that is what pkg/lowl does and what pkg/ml1 runs. The other is to implement
// L and translate the logic. This package is the front end of that second
// route: it scans, parses, and resolves names. pkg/l/lmap is the back end
// behind it, and maps what this produces into LOWL.
//
// The stages mirror the LOWL ones and differ where L does:
//
//	source
//	  -> scanner (pkg/l/scanner)  bytes -> tokens
//	  -> cst     (pkg/l/cst)      tokens -> one node per source line
//	  -> ast     (pkg/l/ast)      lines -> a nested tree of typed statements
//	  -> sema    (pkg/l/sema)     the structural rules, then the names
//
// Nothing here writes a file or touches a process stream. Parse is a function
// of bytes, every listing takes an io.Writer, and cmd/macl and cmd/lcheck are
// the only places that open anything.
package l

import (
	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/cst"
	"github.com/mdhender/maclo/pkg/l/scanner"
	"github.com/mdhender/maclo/pkg/l/sema"
	"github.com/mdhender/maclo/pkg/l/token"
)

// Result is everything a run of the front end produced. The tree and the table
// are returned even when there are diagnostics, because the stages accumulate
// rather than stopping and a partial answer is what makes a listing useful
// while a file is still being fixed.
type Result struct {
	Tokens  []token.Token
	File    *cst.File
	Program *ast.Program
	Table   *sema.Table
	Errs    token.Errors
}

// Parse runs the whole front end over source held in memory.
//
// It exists for the same reason cst.ParseBuffer does in pkg/lowl: the tests
// and any library caller drive the stages from a buffer, and neither names a
// file nor writes one.
func Parse(src []byte) *Result {
	r := &Result{}

	toks, errs := scanner.Scan(src)
	r.Tokens = toks
	r.Errs.Merge(errs)

	file, errs := cst.Parse(toks)
	r.File = file
	r.Errs.Merge(errs)

	prog, errs := ast.Build(file)
	r.Program = prog
	r.Errs.Merge(errs)

	table, errs := sema.Check(prog)
	r.Table = table
	r.Errs.Merge(errs)

	return r
}

// HasErrors reports whether anything of severity error was found. Warnings do
// not count: an identifier longer than the manual allows is worth saying and
// is not a reason to fail.
func (r *Result) HasErrors() bool { return r.Errs.HasErrors() }
