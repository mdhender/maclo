// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mdhender/maclo/internal/fetch"
	"github.com/mdhender/maclo/pkg/ml1"
)

// The bootstrap.
//
// This program is a processor with no processor in it. ML/I is distributed as
// LOWL source, pkg/ml1 runs that source rather than a translation of it, and
// the licence forbids redistributing a machine readable copy — so the engine
// can be neither committed nor embedded, and a freshly installed ml1 has
// nothing to run until the file is on the machine.
//
// Leaving that to the reader means every installation starts with a download,
// an unpack, and a guess at where to put the result. --fetch-engine does it
// instead: one command, verified against the digests in internal/fetch, into
// the directory ml1.EnginePaths already looks in. --engine is the other half,
// and answers the question that follows a failure — which of those places was
// looked at, and what was found.

// showEngine reports where the engine is looked for and which file answers.
func showEngine(w io.Writer) int {
	paths := ml1.EnginePaths()
	found := ""
	for _, p := range paths {
		mark := "     "
		if _, err := os.Stat(p); err == nil {
			if found == "" {
				found, mark = p, "  ->"
			} else {
				mark = "   ." // present, but the search stops before here
			}
		}
		_, _ = fmt.Fprintf(w, "%s %s\n", mark, p)
	}
	if found == "" {
		_, _ = fmt.Fprintf(w, "\nno LOWL source of ML/I found; run ml1 --fetch-engine\n")
		return 1
	}
	_, _ = fmt.Fprintf(w, "\nusing %s\n", found)
	return 0
}

// fetchEngine downloads the LOWL source of ML/I into the per-user directory.
//
// Progress goes to stdout and diagnostics to stderr, which is the shape of
// every other command here. Nothing is written until every member has matched
// the digests recorded in the manifest, so a failure leaves the machine as it
// was rather than half-installed.
func fetchEngine(stdout, stderr io.Writer) int {
	dir := ml1.EngineDir()
	if dir == "" {
		_, _ = fmt.Fprintf(stderr, "ml1: no home directory to install into; set %s\n", ml1.HomeEnv)
		return 1
	}

	m, err := fetch.Load()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "ml1: %v\n", err)
		return 1
	}
	engine, err := m.Find(fetch.EngineCorpus)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "ml1: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "fetching %s\n", engine.URL)
	if err := engine.Install(dir, fetch.Options{
		Progress:  stdout,
		UserAgent: "ml1 (github.com/mdhender/maclo)",
	}); err != nil {
		_, _ = fmt.Fprintf(stderr, "ml1: %v\n", err)
		return 1
	}

	// The archive carries the processor and upstream's own MANIFEST. Naming
	// the file that matters saves the reader working out which is which, and
	// confirms it landed where the search will look for it.
	_, _ = fmt.Fprintf(stdout, "engine ready: %s\n", filepath.Join(dir, ml1.EngineFile))
	return 0
}
