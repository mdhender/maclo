// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package fetch

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Options control one call to Install.
type Options struct {
	// Cache is where downloaded archives are kept, so that a second fetch of
	// the same thing costs nothing. An empty string means no cache at all.
	Cache string

	// VerifyOnly checks what is on disk and never uses the network.
	VerifyOnly bool

	// Force downloads again even when what is on disk already verifies.
	Force bool

	// Progress receives one line per archive. A nil Writer says nothing; this
	// package never reaches for the process's own streams.
	Progress io.Writer

	// UserAgent identifies the caller to the server. Empty means the default.
	UserAgent string
}

const defaultUserAgent = "maclo (github.com/mdhender/maclo)"

// Install brings one archive up to date in target.
//
// Nothing is written until every member has been checked against the manifest,
// so a corrupt or unexpected archive can never leave a half-populated
// directory behind.
func (a *Archive) Install(target string, opt Options) error {
	// Refuse to write anywhere git would track the result. The upstream
	// licence makes this the one mistake that must be impossible to make by
	// accident, so it is enforced here rather than left to a convention, and
	// per archive rather than once, because they do not all land in the same
	// place.
	if err := requireUntracked(target); err != nil {
		return err
	}

	if opt.Force && opt.VerifyOnly {
		return errors.New("force and verify-only cannot be used together")
	}

	// the cheap path first: if what is already on disk matches the manifest
	// there is nothing to do, and nothing touches the network
	if !opt.Force {
		if files, err := readDir(target); err == nil {
			if err := verify(a, files); err == nil {
				progressf(opt.Progress, "%s: up to date (%d files)\n", a.Name, len(files))
				return nil
			} else if opt.VerifyOnly {
				return err
			}
		} else if opt.VerifyOnly {
			return fmt.Errorf("%s: %w", a.Name, err)
		}
	}

	data, err := a.bytes(opt)
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
	progressf(opt.Progress, "%s: %d files in %s\n", a.Name, len(files), target)
	return nil
}

// bytes returns the archive, from the cache when its hash matches and from the
// network otherwise. Nothing unverified is ever returned.
func (a *Archive) bytes(opt Options) ([]byte, error) {
	var cached string
	if opt.Cache != "" {
		cached = filepath.Join(opt.Cache, filepath.Base(a.URL))
		if b, err := os.ReadFile(cached); err == nil {
			if sum := digest(b); sum == a.SHA256 {
				return b, nil
			}
			// a stale or damaged cache entry is not an error; it is just ignored
		}
	}
	if opt.VerifyOnly {
		if cached == "" {
			return nil, fmt.Errorf("%s: verify-only forbids the network and there is no cache", a.Name)
		}
		return nil, fmt.Errorf("%s: not in the cache at %s and verify-only forbids the network", a.Name, cached)
	}

	agent := opt.UserAgent
	if agent == "" {
		agent = defaultUserAgent
	}
	b, err := download(a.URL, agent)
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

	if cached != "" {
		if err := os.MkdirAll(opt.Cache, 0755); err != nil {
			return nil, err
		}
		// written only after the hash matched, so the cache never holds
		// anything unverified
		if err := os.WriteFile(cached, b, 0644); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func download(url, agent string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", agent)

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

// requireUntracked refuses a destination that git would track.
//
// There are two ways to be safe and the walk up the tree looks for both. A
// .gitignore whose first line is * is the pattern this repository uses for
// every directory holding material that may not be redistributed, and it is
// how a developer's checkout qualifies. Being outside a git working tree
// altogether is the other, and it is how the per-user directory an installed
// ml1 fetches the engine into qualifies — there is nothing there to commit to.
//
// The walk stops at the first .git it meets without having seen such a
// .gitignore, because at that point the destination is inside a repository
// that would offer the files for commit.
func requireUntracked(dir string) error {
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
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return fmt.Errorf(`%s is inside the git working tree at %s and is not ignored
The upstream material may not be committed, so it is only written into a
directory covered by a .gitignore whose first line is *, or into one that is
not in a repository at all`, abs, d)
		}
		parent := filepath.Dir(d)
		if parent == d {
			return nil // never in a repository: nothing can track it
		}
		d = parent
	}
}

// InstallEngines copies every .lwl file in src into dst and reports how many
// it wrote.
//
// dst is the directory //go:embed compiles into the binary, which makes this
// the step that turns fetched material into a build input rather than a file
// found at run time. It is a copy rather than the unpack writing there
// directly because the two directories mean different things: src holds the
// archive as upstream published it, digests and all, and dst holds only what
// is meant to end up inside a program. Copying the archive wholesale would
// build upstream's MANIFEST into the binary as well.
//
// dst is held to the same rule as any other destination: it must be somewhere
// git will not offer the result for commit.
func InstallEngines(src, dst string, opt Options) (int, error) {
	if opt.VerifyOnly {
		return 0, nil
	}
	if err := requireUntracked(dst); err != nil {
		return 0, err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", src, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".lwl") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return 0, fmt.Errorf("%s: no .lwl files to install", src)
	}
	sortStrings(names)

	if err := os.MkdirAll(dst, 0755); err != nil {
		return 0, err
	}
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			return 0, err
		}
		if err := os.WriteFile(filepath.Join(dst, name), b, 0644); err != nil {
			return 0, err
		}
		progressf(opt.Progress, "engine: %s in %s\n", name, dst)
	}
	return len(names), nil
}

// Describe reports what a manifest entry for an archive file would look like,
// for a human to review and paste in. It never edits manifest.json, because
// the whole point of the digests is that a change upstream needs a person to
// agree to it.
func Describe(name string, data []byte) (*Archive, error) {
	format := "tar.gz"
	if strings.HasSuffix(name, ".zip") {
		format = "zip"
	}
	files, err := unpack(format, data)
	if err != nil {
		return nil, err
	}

	var names []string
	for n := range files {
		names = append(names, n)
	}
	sortStrings(names)

	base := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(name), ".tar.gz"), ".zip")
	a := &Archive{
		Name:   base,
		URL:    "https://www.ml1.org.uk/" + map[string]string{"tar.gz": "tgz", "zip": "zip"}[format] + "/" + filepath.Base(name),
		Format: format,
		SHA256: digest(data),
		Size:   int64(len(data)),
		Dest:   base,
	}
	for _, n := range names {
		a.Files = append(a.Files, Member{Name: n, Size: int64(len(files[n])), SHA256: digest(files[n])})
	}
	return a, nil
}

func progressf(w io.Writer, format string, args ...any) {
	if w != nil {
		_, _ = fmt.Fprintf(w, format, args...)
	}
}
