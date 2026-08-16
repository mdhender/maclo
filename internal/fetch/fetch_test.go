// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package fetch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManifest checks the embedded manifest offline, so a typo in it is a
// test failure rather than something discovered on a machine with no network.
func TestManifest(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
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
	a, err := m.Find("tests-ac")
	if err != nil {
		t.Fatalf("Find: %v", err)
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
	engine, err := m.Find(EngineCorpus)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if engine.Optional {
		t.Errorf("%s: want it fetched by default; the golden tests skip without it", EngineCorpus)
	}
	if got, want := engine.Target("/repo", "/repo/testdata/upstream"), filepath.Join("/repo", ".downloads", EngineCorpus); got != want {
		t.Errorf("%s: Target: want %s: got %s", EngineCorpus, want, got)
	}
	var hasSource bool
	for _, f := range engine.Files {
		if f.Name == EngineFile {
			hasSource = true
		}
	}
	if !hasSource {
		t.Errorf("%s: want %s among the members, got none", EngineCorpus, EngineFile)
	}
}

// TestSelect covers the three ways a caller names what it wants. cmd/ml1 asks
// for the engine by name and must not drag the test suite along with it.
func TestSelect(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	all, err := m.Select("all")
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(all) != len(m.Corpora) {
		t.Errorf(`Select("all"): want %d, got %d`, len(m.Corpora), len(all))
	}

	required, err := m.Select("required")
	if err != nil {
		t.Fatalf("required: %v", err)
	}
	if len(required) == 0 || len(required) > len(all) {
		t.Errorf(`Select("required"): want between 1 and %d, got %d`, len(all), len(required))
	}
	if empty, err := m.Select(""); err != nil || len(empty) != len(required) {
		t.Errorf(`Select(""): want the same as "required" (%d), got %d (%v)`, len(required), len(empty), err)
	}

	one, err := m.Select(EngineCorpus)
	if err != nil {
		t.Fatalf("%s: %v", EngineCorpus, err)
	}
	if len(one) != 1 || one[0].Name != EngineCorpus {
		t.Errorf("Select(%q): want just that one, got %d", EngineCorpus, len(one))
	}

	if _, err := m.Select("no-such-corpus"); err == nil {
		t.Errorf("Select of an unknown name: want an error, got nil")
	}
}

// TestArchiveTarget covers the two places an archive may be unpacked into.
// Test data goes under the corpora destination, which -dest may move; anything
// that is not test data names its own directory under the module root, because
// something else goes looking for it there.
func TestArchiveTarget(t *testing.T) {
	corpus := Archive{Name: "tests-ac", Dest: "tests-ac"}
	if got, want := corpus.Target("/repo", "/elsewhere"), filepath.Join("/elsewhere", "tests-ac"); got != want {
		t.Errorf("corpus: want %s: got %s", want, got)
	}

	engine := Archive{Name: "lowlml1", Dest: "lowlml1", Under: ".downloads"}
	if got, want := engine.Target("/repo", "/elsewhere"), filepath.Join("/repo", ".downloads", "lowlml1"); got != want {
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

// TestRequireUntracked covers the gate that makes it impossible to drop the
// upstream material somewhere git would offer it for commit.
//
// The gate has two ways to say yes and they are not the same case. A
// developer's checkout qualifies because .downloads carries a .gitignore whose
// first line is *; the per-user directory an installed ml1 fetches the engine
// into qualifies because it is not in a repository at all. The tests build a
// repository out of a bare .git directory, which is all the walk looks for.
func TestRequireUntracked(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	tracked := filepath.Join(repo, "tracked", "deep")
	if err := os.MkdirAll(tracked, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := requireUntracked(tracked); err == nil {
		t.Errorf("tracked directory: want an error, got nil")
	}

	ignored := filepath.Join(repo, "ignored")
	if err := os.MkdirAll(filepath.Join(ignored, "deep"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ignored, ".gitignore"), []byte("*\n!.gitignore\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// the gate looks up the tree, so a directory below the .gitignore counts
	if err := requireUntracked(filepath.Join(ignored, "deep")); err != nil {
		t.Errorf("ignored directory: want nil error, got %v", err)
	}

	// a .gitignore that does not ignore everything is not good enough
	partial := filepath.Join(repo, "partial")
	if err := os.MkdirAll(partial, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(partial, ".gitignore"), []byte("*.out\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := requireUntracked(partial); err == nil {
		t.Errorf("partial .gitignore: want an error, got nil")
	}

	// Outside a repository there is nothing to track the files, which is the
	// case cmd/ml1 --fetch-engine relies on: the per-user directory is not a
	// checkout and never will be. A directory that does not exist yet is the
	// normal state on a first fetch, so it is asked about too.
	if err := requireUntracked(filepath.Join(root, "elsewhere")); err != nil {
		t.Errorf("outside a repository: want nil error, got %v", err)
	}
	if err := requireUntracked(filepath.Join(root, "not", "created", "yet")); err != nil {
		t.Errorf("a directory that does not exist yet: want nil error, got %v", err)
	}

	// The gate is applied to each archive's own target rather than once to the
	// corpora directory, because they do not all land in the same place: an
	// entry that named a tracked directory would otherwise slip past a check
	// made somewhere else.
	err := (&Archive{Name: "x", Dest: "x"}).Install(tracked, Options{VerifyOnly: true})
	if err == nil {
		t.Errorf("Install into a tracked directory: want an error, got nil")
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

// TestVerifyOnlyNeverReachesTheNetwork checks the flag that makes a fetch
// offline. It is what the golden tests' "is my checkout intact" path uses, and
// what must not silently start downloading on a machine with no network.
func TestVerifyOnlyNeverReachesTheNetwork(t *testing.T) {
	// an empty target cannot verify — the member the archive promises is not
	// there — and with no cache there is nowhere left to get the archive from
	// except the network, which the flag forbids
	a := &Archive{
		Name: "x", Dest: "x", Format: "tar.gz", URL: "https://example.invalid/x.tar.gz",
		Files: []Member{{Name: "x.lwl", Size: 1, SHA256: digest([]byte("x"))}},
	}
	err := a.Install(t.TempDir(), Options{VerifyOnly: true})
	if err == nil {
		t.Fatalf("verify-only against an empty directory: want an error, got nil")
	}
	// The url is unresolvable, so an implementation that reached for the
	// network would fail with a DNS error instead. Naming the member that is
	// missing is what proves the check was made against the disk alone.
	if !strings.Contains(err.Error(), "x.lwl: missing") {
		t.Errorf("want the error to name the missing member, got %v", err)
	}

	// and it is refused outright alongside Force, rather than one quietly
	// winning over the other
	if err := a.Install(t.TempDir(), Options{VerifyOnly: true, Force: true}); err == nil {
		t.Errorf("verify-only with force: want an error, got nil")
	}
}
