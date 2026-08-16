// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

// Command fetchtestdata downloads from ml1.org.uk everything the tests in
// pkg/ml1 need and this repository may not carry: the ML/I test suite, and the
// LOWL source of ML/I itself, which is the processor those tests run.
//
// Neither is in this repository on purpose. Both are copyright P.J. Brown and
// R.D. Eager, and the licence forbids making a machine readable copy generally
// accessible. What is committed instead is internal/fetch/manifest.json, which
// records the URL, the sizes, and the SHA-256 of every file: facts about them
// rather than copies of them. Everything this command writes is verified
// against those digests, so a corrupt download or a change upstream is
// reported rather than absorbed, and it will only write into a directory git
// ignores.
//
// The two land in different places, because they are different kinds of thing.
// The suite is test data and goes under testdata/upstream; the LOWL source is
// the engine and goes where the engine looks for it, in .downloads.
//
// This is the developer's command, and it populates a checkout. An installed
// ml1 has no checkout to populate, so it fetches the engine alone into a
// per-user directory with `ml1 --fetch-engine`; both go through internal/fetch.
//
// Usage:
//
//	go run ./cmd/fetchtestdata              fetch and verify what is missing
//	go run ./cmd/fetchtestdata -verify      check what is on disk, no network
//	go run ./cmd/fetchtestdata -force       fetch again even if it verifies
//	go run ./cmd/fetchtestdata -corpus all  include the optional archives
//	go run ./cmd/fetchtestdata -print-manifest f.tar.gz
//	                                        print a manifest entry for an
//	                                        archive, to be reviewed by hand
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/maloquacious/ml_i/internal/fetch"
)

func main() {
	var (
		dest       = flag.String("dest", "", "where to put the corpora (default: testdata/upstream under the module root)")
		cache      = flag.String("cache", "", "where to keep downloaded archives (default: .downloads/cache under the module root)")
		corpus     = flag.String("corpus", "required", `which corpora to handle: a name, "required", or "all"`)
		verifyOnly = flag.Bool("verify", false, "check what is on disk and never use the network")
		force      = flag.Bool("force", false, "download again even when the corpus already verifies")
		printFor   = flag.String("print-manifest", "", "print a manifest entry for the named archive file and exit")
		engines    = flag.String("engines", "", "where to put the LOWL sources that get embedded (default: pkg/ml1/engines under the module root)")
	)
	flag.Parse()

	if err := realMain(*dest, *cache, *corpus, *printFor, *engines, *verifyOnly, *force); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fetchtestdata: %v\n", err)
		os.Exit(1)
	}
}

func realMain(dest, cache, corpus, printFor, engines string, verifyOnly, force bool) error {
	// before the manifest is loaded, so that this can be used to bootstrap a
	// manifest that does not exist yet or no longer parses
	if printFor != "" {
		return printManifest(printFor)
	}

	m, err := fetch.Load()
	if err != nil {
		return err
	}

	root := moduleRoot()
	if dest == "" {
		dest = filepath.Join(root, "testdata", "upstream")
	}
	if cache == "" {
		cache = filepath.Join(root, ".downloads", "cache")
	}

	chosen, err := m.Select(corpus)
	if err != nil {
		return err
	}

	opt := fetch.Options{
		Cache:      cache,
		VerifyOnly: verifyOnly,
		Force:      force,
		Progress:   os.Stdout,
		UserAgent:  "fetchtestdata (github.com/maloquacious/ml_i)",
	}
	for _, a := range chosen {
		target := a.Target(root, dest)
		if err := a.Install(target, opt); err != nil {
			return err
		}
		// The engine is not only test data. Copying it into the embed
		// directory is what lets it be compiled into cmd/maclo, and doing it
		// here rather than leaving it to the reader is the difference between
		// a checkout that builds a working processor and one that builds an
		// empty one.
		if a.Name == fetch.EngineCorpus {
			if engines == "" {
				engines = filepath.Join(root, "pkg", "ml1", "engines")
			}
			if _, err := fetch.InstallEngines(target, engines, opt); err != nil {
				return err
			}
		}
	}
	return nil
}

// printManifest reports what a manifest entry for an archive file would look
// like. It writes to the standard output for a human to review and paste in;
// it never edits manifest.json, because the whole point of the digests is
// that a change upstream needs a person to agree to it.
func printManifest(name string) error {
	b, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	a, err := fetch.Describe(name, b)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("    ", "  ")
	return enc.Encode(a)
}

// moduleRoot walks up from the working directory looking for go.mod.
func moduleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
