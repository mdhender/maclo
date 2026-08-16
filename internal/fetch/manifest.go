// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

// Package fetch downloads and verifies the material from ml1.org.uk that this
// repository may not carry: the ML/I test suite, and the LOWL source of ML/I
// itself, which is the processor.
//
// Neither is here on purpose. Both are copyright P.J. Brown and R.D. Eager,
// and the licence forbids making a machine readable copy generally accessible.
// What is committed instead is manifest.json, which records the URL, the sizes,
// and the SHA-256 of every file: facts about them rather than copies of them.
//
// Two commands use this. cmd/fetchtestdata brings a developer's checkout up to
// date, and cmd/ml1 fetches the engine alone into a per-user directory so that
// an installed binary has something to run. They differ only in what they ask
// for and where they put it, which is why the machinery is here rather than in
// either of them.
//
// Nothing in this package writes to the process's streams. Progress goes to
// the io.Writer in Options, and everything else comes back as a value or an
// error, because a package that printed would be unusable from the other one.
package fetch

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
)

const (
	// EngineCorpus is the manifest entry holding the newest published LOWL
	// source of ML/I. Several entries carry an engine — see Archive.Engine —
	// and this is the one to fetch when only one is wanted, which is what
	// `ml1 --fetch-engine` does for a user with no checkout.
	EngineCorpus = "lowlml1"

	// EngineFile is the member of that archive which is the processor.
	EngineFile = "ml1ajb.lwl"
)

// manifest.json records where each upstream archive comes from and what it
// should contain.
//
// It is committed, and that is deliberate. The archives themselves may not be
// redistributed, but a file name, a size, and a SHA-256 are facts about a
// file rather than a copy of it: a digest is thirty-two one-way bytes and
// nothing can be reconstructed from it. Keeping the digests here is what lets
// a fetch be verified, and what makes a change upstream visible instead of
// silent.
//
//go:embed manifest.json
var manifestJSON []byte

// Manifest is the whole file.
type Manifest struct {
	Version int       `json:"version"`
	Corpora []Archive `json:"corpora"`
}

// Archive is one downloadable suite.
type Archive struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Format string `json:"format"` // "tar.gz" or "zip"
	SHA256 string `json:"sha256"` // of the archive as downloaded
	Size   int64  `json:"size"`
	Dest   string `json:"dest"` // relative to the destination directory

	// Under overrides the destination directory, relative to the module root.
	// Not everything worth fetching is test data: the LOWL source of ML/I is
	// the processor itself, and the engine looks for it in .downloads, so it
	// has a fixed home rather than one the -dest flag may move.
	Under string `json:"under,omitempty"`

	// Optional keeps an archive out of the default fetch. The zip suite is
	// an older generation of the same tests rather than a re-encoding of the
	// tar one, so it is not interchangeable and does not gate anything.
	Optional bool `json:"optional,omitempty"`

	// Engine marks an archive whose .lwl members are LOWL sources of ML/I, to
	// be installed where //go:embed will compile them in as well as unpacked.
	//
	// It is a property of the entry rather than a name matched in code because
	// there is more than one version of ML/I and there will be more: naming
	// one of them in a condition made the newest source the only one a build
	// could ever carry, which is exactly what the embedding was for.
	Engine bool `json:"engine,omitempty"`

	// Files is every member the archive should yield, so that a partial
	// extraction or a later hand-edit is detectable.
	Files []Member `json:"files"`
}

// Member is one file inside an archive.
type Member struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Load parses the embedded manifest and checks the parts of it that a typo
// could break.
func Load() (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(manifestJSON, &m); err != nil {
		return nil, fmt.Errorf("manifest.json: %w", err)
	}
	if len(m.Corpora) == 0 {
		return nil, fmt.Errorf("manifest.json: no corpora")
	}
	for i := range m.Corpora {
		a := &m.Corpora[i]
		switch {
		case a.Name == "":
			return nil, fmt.Errorf("manifest.json: corpus %d has no name", i)
		case a.URL == "":
			return nil, fmt.Errorf("manifest.json: %s has no url", a.Name)
		case a.SHA256 == "":
			return nil, fmt.Errorf("manifest.json: %s has no sha256", a.Name)
		case a.Dest == "":
			return nil, fmt.Errorf("manifest.json: %s has no dest", a.Name)
		case a.Format != "tar.gz" && a.Format != "zip":
			return nil, fmt.Errorf("manifest.json: %s: unknown format %q", a.Name, a.Format)
		}
		if a.Under != "" {
			if err := safeName(a.Under); err != nil {
				return nil, fmt.Errorf("manifest.json: %s: under: %w", a.Name, err)
			}
		}
		if err := safeName(a.Dest); err != nil {
			return nil, fmt.Errorf("manifest.json: %s: dest: %w", a.Name, err)
		}
		seen := make(map[string]bool, len(a.Files))
		for _, f := range a.Files {
			if err := safeName(f.Name); err != nil {
				return nil, fmt.Errorf("manifest.json: %s: %w", a.Name, err)
			}
			if seen[f.Name] {
				return nil, fmt.Errorf("manifest.json: %s: %s listed twice", a.Name, f.Name)
			}
			seen[f.Name] = true
		}
	}
	return &m, nil
}

// Target is where this archive's files belong: under the module root when the
// entry names its own directory, and under the corpora destination otherwise.
func (a *Archive) Target(root, dest string) string {
	if a.Under != "" {
		return filepath.Join(root, filepath.FromSlash(a.Under), a.Dest)
	}
	return filepath.Join(dest, a.Dest)
}

// Find returns the archive with the given name.
func (m *Manifest) Find(name string) (*Archive, error) {
	for i := range m.Corpora {
		if m.Corpora[i].Name == name {
			return &m.Corpora[i], nil
		}
	}
	return nil, fmt.Errorf("no corpus named %q in the manifest", name)
}

// Select resolves the name of a set of archives: one corpus by name, "all",
// or "required", which is everything not marked optional.
func (m *Manifest) Select(which string) ([]*Archive, error) {
	var chosen []*Archive
	switch which {
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
		a, err := m.Find(which)
		if err != nil {
			return nil, err
		}
		chosen = append(chosen, a)
	}
	return chosen, nil
}
