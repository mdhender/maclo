// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package sema

import (
	"bufio"
	"fmt"
	"io"
)

// WriteListing writes the symbol table: the names a program declares, in the
// order it declares them, then the names it used and never declared.
//
// Like every listing in pkg/l this takes an io.Writer rather than naming a
// file, so no artifact name exists outside cmd and debug_artifacts_test.go has
// nothing new to declare.
func WriteListing(w io.Writer, t *Table) error {
	bw := bufio.NewWriter(w)
	fmt.Fprintf(bw, "%-10s %-14s %-4s %-9s %-6s %-7s %s\n",
		"NAME", "KIND", "TYPE", "SECTION", "BLOCK", "DEF", "USES")
	for _, s := range t.Symbols() {
		if s.Predefined && len(s.Refs) == 0 {
			continue // do not list the whole vocabulary of L on every run
		}
		fmt.Fprintf(bw, "%-10s %-14s %-4s %-9s %-6s %-7s %d\n",
			s.Name, s.Kind, dash(s.typeText()), dash(s.Section), dash(s.Block), dash(s.defText()), len(s.Refs))
	}
	return bw.Flush()
}

func (s *Symbol) typeText() string {
	switch s.Kind {
	case Variable:
		return s.Type.String()
	}
	return ""
}

func (s *Symbol) defText() string {
	if s.Predefined {
		return "(L)"
	}
	if !s.Def.IsValid() {
		return ""
	}
	return s.Def.String()
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
