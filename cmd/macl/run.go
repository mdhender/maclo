// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/mdhender/maclo/pkg/l"
)

// runProgram is the verb this command is named for and the one it cannot do.
//
// It exists now rather than later for two reasons. Reserving the word means
// `macl ml1aie.l` cannot quietly come to mean something else first, and a user
// who types the obvious thing gets told where the gap is instead of a usage
// message that never mentions it.
//
// It is not a stub that refuses to look at the file. Everything short of code
// generation works, so it runs the front end and reports it: if the program
// will not resolve, that is the answer, and it is the same answer it would be
// with a back end behind it. Only when the program is clean does the missing
// half become the reason it stopped.
func runProgram(e *env, args []string) int {
	o, code, ok := parse(e, "run", args, false)
	if !ok {
		return code
	}
	src, err := os.ReadFile(o.source)
	if err != nil {
		fmt.Fprintf(e.stderr, "macl run: %v\n", err)
		return exitUsage
	}

	result := l.Parse(src)
	diagnose(e.stderr, o, result)
	if result.HasErrors() {
		fmt.Fprintf(e.stderr, "macl run: %s does not resolve, so there is nothing to run\n", o.source)
		return exitErrors
	}

	fmt.Fprintf(e.stderr, noBackEnd, o.source)
	return exitNoBackEnd
}

// noBackEnd says what is missing and what to do instead. It takes the source
// name so that the commands it suggests are ones the reader can paste.
const noBackEnd = `macl run: there is no back end for L yet

pkg/l is a front end: it scans, parses and resolves names, and stops there.
What does not exist is the half that turns the tree into something a machine
executes, so there is nothing here to run an L program with.

What macl can tell you about this one now:

    macl summary %[1]s
    macl list    %[1]s
    macl symbols %[1]s

To run ML/I today, take the other porting route, which is finished:

    maclo file.ml1          an engine built into the binary
    ml1 file.ml1            the operating instructions of Appendix AA
`
