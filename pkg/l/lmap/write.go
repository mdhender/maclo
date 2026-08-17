// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package lmap

import (
	"bufio"
	"io"
	"strings"
)

// The canonical form.
//
// LOWL's own layout is free: pkg/lowl/cst drops commas, ignores leading
// whitespace, and takes one statement per line and nothing else from the
// column a thing sits in. So the shape below is a choice, and it is made for
// the same reason ast.WriteListing's is -- the output is compared byte for
// byte in a golden file, and a rendering with one degree of freedom in it is a
// rendering that will one day differ from itself.
const (
	indent  = "        " // eight spaces before every mnemonic
	opWidth = 8          // mnemonics are padded to here, the longest being five
)

// WriteLOWL renders the program as the text an assembler reads.
//
// It takes an io.Writer rather than a file name because nothing under pkg/l
// opens a file. pkg/ml1 hands it a bytes.Buffer and assembles out of that;
// cmd/macl hands it a path the user named.
func (p *Program) WriteLOWL(w io.Writer) error {
	bw := bufio.NewWriter(w)
	for _, s := range p.Stmts {
		if s.Op == "" {
			// a label, or a blank line
			if s.Label != "" {
				bw.WriteString("[")
				bw.WriteString(s.Label)
				bw.WriteString("]")
			}
			bw.WriteByte('\n')
			continue
		}
		bw.WriteString(indent)
		bw.WriteString(s.Op)
		if len(s.Args) != 0 {
			if pad := opWidth - len(s.Op); pad > 0 {
				bw.WriteString(strings.Repeat(" ", pad))
			} else {
				bw.WriteByte(' ')
			}
			for i, a := range s.Args {
				if i > 0 {
					bw.WriteByte(',')
				}
				bw.WriteString(a.String())
			}
		}
		bw.WriteByte('\n')
	}
	return bw.Flush()
}
