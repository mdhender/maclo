// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

// Command fetchtestdata downloads from ml1.org.uk everything the tests in
// pkg/ml1 need and this repository may not carry: the ML/I test suite, and the
// LOWL source of ML/I itself, which is the processor those tests run.
//
// Neither is in this repository on purpose. Both are copyright P.J. Brown and
// R.D. Eager, and the licence forbids making a machine readable copy generally
// accessible. What is committed instead is manifest.json, which records the
// URL, the sizes, and the SHA-256 of every file: facts about them rather than
// copies of them. Everything this command writes is verified against those
// digests, so a corrupt download or a change upstream is reported rather than
// absorbed, and it will only write into a directory git ignores.
//
// The two land in different places, because they are different kinds of thing.
// The suite is test data and goes under testdata/upstream; the LOWL source is
// the engine and goes where the engine looks for it, in .downloads.
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
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	var (
		dest       = flag.String("dest", "", "where to put the corpora (default: testdata/upstream under the module root)")
		cache      = flag.String("cache", "", "where to keep downloaded archives (default: .downloads/cache under the module root)")
		corpus     = flag.String("corpus", "required", `which corpora to handle: a name, "required", or "all"`)
		verifyOnly = flag.Bool("verify", false, "check what is on disk and never use the network")
		force      = flag.Bool("force", false, "download again even when the corpus already verifies")
		printFor   = flag.String("print-manifest", "", "print a manifest entry for the named archive file and exit")
	)
	flag.Parse()

	if err := realMain(*dest, *cache, *corpus, *printFor, *verifyOnly, *force); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fetchtestdata: %v\n", err)
		os.Exit(1)
	}
}

func realMain(dest, cache, corpus, printFor string, verifyOnly, force bool) error {
	// before the manifest is loaded, so that this can be used to bootstrap a
	// manifest that does not exist yet or no longer parses
	if printFor != "" {
		return printManifest(printFor)
	}

	m, err := loadManifest()
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

	var chosen []*Archive
	switch corpus {
	case "all":
		for i := range m.Corpora {
			chosen = append(chosen, &m.Corpora[i])
		}
	case "required", "":
		for i := range m.Corpora {
			if !m.Corpora[i].Optional {
				chosen = append(chosen, &m.Corpora[i])
			}
		}
	default:
		a, err := m.find(corpus)
		if err != nil {
			return err
		}
		chosen = append(chosen, a)
	}

	for _, a := range chosen {
		if err := handle(a, a.target(root, dest), cache, verifyOnly, force); err != nil {
			return err
		}
	}
	return nil
}

// handle brings one corpus up to date.
func handle(a *Archive, target, cache string, verifyOnly, force bool) error {
	// Refuse to write anywhere git would track the result. The upstream
	// licence makes this the one mistake that must be impossible to make by
	// accident, so it is enforced here rather than left to a convention, and
	// per archive rather than once, because they do not all land in the same
	// place.
	if err := requireIgnored(target); err != nil {
		return err
	}

	// the cheap path first: if what is already on disk matches the manifest
	// there is nothing to do, and nothing touches the network
	if !force {
		if files, err := readDir(target); err == nil {
			if err := verify(a, files); err == nil {
				fmt.Printf("%s: up to date (%d files)\n", a.Name, len(files))
				return nil
			} else if verifyOnly {
				return err
			}
		} else if verifyOnly {
			return fmt.Errorf("%s: %w", a.Name, err)
		}
	} else if verifyOnly {
		return errors.New("-force and -verify cannot be used together")
	}

	data, err := archiveBytes(a, cache, verifyOnly)
	if err != nil {
		return err
	}

	files, err := unpack(a.Format, data)
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name, err)
	}
	if err := verify(a, files); err != nil {
		return err
	}

	// only now, with every member checked, is anything written
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	for name, b := range files {
		out := filepath.Join(target, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(out, b, 0644); err != nil {
			return err
		}
	}
	fmt.Printf("%s: %d files in %s\n", a.Name, len(files), target)
	return nil
}

// archiveBytes returns the archive, from the cache when its hash matches and
// from the network otherwise. Nothing unverified is ever returned.
func archiveBytes(a *Archive, cache string, offline bool) ([]byte, error) {
	cached := filepath.Join(cache, filepath.Base(a.URL))
	if b, err := os.ReadFile(cached); err == nil {
		if sum := digest(b); sum == a.SHA256 {
			return b, nil
		}
		// a stale or damaged cache entry is not an error; it is just ignored
	}
	if offline {
		return nil, fmt.Errorf("%s: not in the cache at %s and -verify forbids the network", a.Name, cached)
	}

	b, err := download(a.URL)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", a.Name, err)
	}
	if sum := digest(b); sum != a.SHA256 {
		return nil, fmt.Errorf(`%s: the archive is not what the manifest describes
  url      %s
  expected %s
  actual   %s
Upstream may have changed, or the download may be damaged. Nothing has been
written. Do not edit manifest.json to make this pass until a human has looked
at what changed; see docs/how-to/fetch-the-upstream-sources.md`,
			a.Name, a.URL, a.SHA256, sum)
	}

	if err := os.MkdirAll(cache, 0755); err != nil {
		return nil, err
	}
	// written only after the hash matched, so the cache never holds anything
	// unverified
	if err := os.WriteFile(cached, b, 0644); err != nil {
		return nil, err
	}
	return b, nil
}

func download(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "fetchtestdata (github.com/maloquacious/ml_i)")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxArchive+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxArchive {
		return nil, fmt.Errorf("%s: larger than the %d byte limit", url, maxArchive)
	}
	return b, nil
}

// readDir reads an extracted corpus back into the same shape unpack produces,
// so that one verify serves both.
func readDir(dir string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	err := filepath.Walk(dir, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// requireIgnored refuses a destination that git would track.
//
// It walks up from dir looking for a .gitignore whose first line is *, which
// is the pattern this repository uses for every directory holding material
// that may not be redistributed.
func requireIgnored(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for d := abs; ; {
		if b, err := os.ReadFile(filepath.Join(d, ".gitignore")); err == nil {
			first := b
			if i := strings.IndexByte(string(b), '\n'); i >= 0 {
				first = b[:i]
			}
			if strings.TrimSpace(string(first)) == "*" {
				return nil
			}
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return fmt.Errorf(`%s is not inside a directory that git ignores
The upstream suite may not be committed, so this command will only write into
a directory covered by a .gitignore whose first line is *`, abs)
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
	format := "tar.gz"
	if strings.HasSuffix(name, ".zip") {
		format = "zip"
	}
	files, err := unpack(format, b)
	if err != nil {
		return err
	}

	var names []string
	for n := range files {
		names = append(names, n)
	}
	sortStrings(names)

	a := Archive{
		Name:   strings.TrimSuffix(strings.TrimSuffix(filepath.Base(name), ".tar.gz"), ".zip"),
		URL:    "https://www.ml1.org.uk/" + map[string]string{"tar.gz": "tgz", "zip": "zip"}[format] + "/" + filepath.Base(name),
		Format: format,
		SHA256: digest(b),
		Size:   int64(len(b)),
		Dest:   strings.TrimSuffix(strings.TrimSuffix(filepath.Base(name), ".tar.gz"), ".zip"),
	}
	for _, n := range names {
		a.Files = append(a.Files, Member{Name: n, Size: int64(len(files[n])), SHA256: digest(files[n])})
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
