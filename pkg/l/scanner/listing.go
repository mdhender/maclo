// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package scanner

import (
	"bufio"
	"fmt"
	"io"

	"github.com/mdhender/maclo/pkg/l/token"
)

// WriteTokens dumps the token stream for a reader.
//
// It takes an io.Writer rather than naming a file the way
// scanner.TestScanner does in pkg/lowl. That keeps every literal artifact
// name in a cmd, so pkg/l never appears in the writeSites table in
// debug_artifacts_test.go and the root .gitignore needs no new entries.
func WriteTokens(w io.Writer, toks []token.Token) error {
	bw := bufio.NewWriter(w)
	for _, t := range toks {
		if _, err := fmt.Fprintf(bw, "%s\n", t); err != nil {
			return err
		}
	}
	return bw.Flush()
}
