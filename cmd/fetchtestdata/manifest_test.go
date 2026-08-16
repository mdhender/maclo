// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManifest checks the embedded manifest offline, so a typo in it is a
// test failure rather than something discovered on a machine with no network.
func TestManifest(t *testing.T) {
	m, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if len(m.Corpora) == 0 {
		t.Fatalf("corpora: want at least one, got none")
	}

	required := 0
	for _, a := range m.Corpora {
		if !a.Optional {
			required++
		}
		if len(a.Files) == 0 {
			t.Errorf("%s: want members, got none", a.Name)
		}
		if !strings.HasPrefix(a.URL, "https://www.ml1.org.uk/") {
			t.Errorf("%s: want an ml1.org.uk url, got %q", a.Name, a.URL)
		}
		if len(a.SHA256) != 64 {
			t.Errorf("%s: want a 64 character sha256, got %d characters", a.Name, len(a.SHA256))
		}
		for _, f := range a.Files {
			if len(f.SHA256) != 64 {
				t.Errorf("%s: %s: want a 64 character sha256, got %d characters", a.Name, f.Name, len(f.SHA256))
			}
			if f.Size < 0 {
				t.Errorf("%s: %s: negative size", a.Name, f.Name)
			}
		}
	}
	if required == 0 {
		t.Errorf("corpora: every corpus is optional, so a plain fetch would do nothing")
	}

	// the suite the golden tests actually use must be there and must carry
	// the prelude that every upstream case is run with
	a, err := m.find("tests-ac")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	var hasPrelude bool
	for _, f := range a.Files {
		if f.Name == "sets18.ml1" {
			hasPrelude = true
		}
	}
	if !hasPrelude {
		t.Errorf("tests-ac: want sets18.ml1 among the members, got none")
	}

	// The engine has to be fetched too, and it has to land where ml1.Run looks
	// for it. Nothing else in the repository would notice if this entry moved:
	// the tests would simply go back to skipping, which reads like an
	// unfetched clone rather than like a broken manifest.
	engine, err := m.find("lowlml1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if engine.Optional {
		t.Errorf("lowlml1: want it fetched by default; the golden tests skip without it")
	}
	if got, want := engine.target("/repo", "/repo/testdata/upstream"), filepath.Join("/repo", ".downloads", "lowlml1"); got != want {
		t.Errorf("lowlml1: target: want %s: got %s", want, got)
	}
	var hasSource bool
	for _, f := range engine.Files {
		if f.Name == "ml1ajb.lwl" {
			hasSource = true
		}
	}
	if !hasSource {
		t.Errorf("lowlml1: want ml1ajb.lwl among the members, got none")
	}
}

// TestArchiveTarget covers the two places an archive may be unpacked into.
// Test data goes under the corpora destination, which -dest may move; anything
// that is not test data names its own directory under the module root, because
// something else goes looking for it there.
func TestArchiveTarget(t *testing.T) {
	corpus := Archive{Name: "tests-ac", Dest: "tests-ac"}
	if got, want := corpus.target("/repo", "/elsewhere"), filepath.Join("/elsewhere", "tests-ac"); got != want {
		t.Errorf("corpus: want %s: got %s", want, got)
	}

	engine := Archive{Name: "lowlml1", Dest: "lowlml1", Under: ".downloads"}
	if got, want := engine.target("/repo", "/elsewhere"), filepath.Join("/repo", ".downloads", "lowlml1"); got != want {
		t.Errorf("engine: want %s: got %s (-dest must not move it)", want, got)
	}
}

// TestSafeName covers the check that keeps an archive from writing outside
// the directory it is extracted into.
func TestSafeName(t *testing.T) {
	for _, tc := range []struct {
		id   int
		name string
		ok   bool
	}{
		{1, "alter.ml1", true},
		{2, "tests/alter.ml1", true},
		{3, "", false},
		{4, "/etc/passwd", false},
		{5, "../escape", false},
		{6, "a/../../escape", false},
		{7, "./alter.ml1", false},
		{8, `a\b`, false},
		{9, "a//b", false},
	} {
		err := safeName(tc.name)
		if tc.ok && err != nil {
			t.Errorf("%d: %q: want nil error, got %v", tc.id, tc.name, err)
		} else if !tc.ok && err == nil {
			t.Errorf("%d: %q: want an error, got nil", tc.id, tc.name)
		}
	}
}

// TestRequireIgnored covers the gate that makes it impossible to drop the
// upstream suite somewhere git would track it.
func TestRequireIgnored(t *testing.T) {
	root := t.TempDir()

	tracked := filepath.Join(root, "tracked", "deep")
	if err := os.MkdirAll(tracked, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := requireIgnored(tracked); err == nil {
		t.Errorf("tracked directory: want an error, got nil")
	}

	ignored := filepath.Join(root, "ignored")
	if err := os.MkdirAll(filepath.Join(ignored, "deep"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ignored, ".gitignore"), []byte("*\n!.gitignore\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// the gate looks up the tree, so a directory below the .gitignore counts
	if err := requireIgnored(filepath.Join(ignored, "deep")); err != nil {
		t.Errorf("ignored directory: want nil error, got %v", err)
	}

	// a .gitignore that does not ignore everything is not good enough
	partial := filepath.Join(root, "partial")
	if err := os.MkdirAll(partial, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(partial, ".gitignore"), []byte("*.out\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := requireIgnored(partial); err == nil {
		t.Errorf("partial .gitignore: want an error, got nil")
	}

	// The gate is applied to each archive's own target rather than once to the
	// corpora directory, because they no longer all land in the same place: an
	// entry that named a tracked directory would otherwise slip past a check
	// made somewhere else.
	if err := handle(&Archive{Name: "x", Dest: "x"}, tracked, t.TempDir(), true, false); err == nil {
		t.Errorf("handle into a tracked directory: want an error, got nil")
	}
}

// TestUnpackRejectsBadMembers checks that a hostile archive is refused rather
// than trusted.
func TestUnpackRejectsBadMembers(t *testing.T) {
	if _, err := unpack("cpio", nil); err == nil {
		t.Errorf("unknown format: want an error, got nil")
	}
	if _, err := unpack("tar.gz", []byte("not a gzip stream")); err == nil {
		t.Errorf("damaged archive: want an error, got nil")
	}
}
