// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package fetch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
)

const (
	// maxArchive and maxMember cap what will be read from the network and
	// from inside an archive. The real suites are about twenty kilobytes, so
	// these are several orders of magnitude of headroom and exist only to
	// stop a hostile or corrupt archive from filling the disk.
	maxArchive = 32 << 20
	maxMember  = 8 << 20
	maxTotal   = 64 << 20
)

// safeName rejects any member name that would escape the directory it is
// extracted into. The fetcher never trusts an archive's idea of where its
// contents belong.
func safeName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("empty member name")
	case path.IsAbs(name) || strings.HasPrefix(name, "/"):
		return fmt.Errorf("%s: absolute member name", name)
	case strings.Contains(name, `\`):
		return fmt.Errorf("%s: backslash in member name", name)
	}
	clean := path.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("%s: member name escapes the destination", name)
	}
	if clean != name {
		return fmt.Errorf("%s: member name is not in canonical form", name)
	}
	return nil
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// unpack reads an archive into memory and returns its regular files.
//
// Nothing is written to disk here: the caller verifies the whole set against
// the manifest first, so a corrupt or unexpected archive can never leave a
// half-populated corpus behind.
func unpack(format string, data []byte) (map[string][]byte, error) {
	switch format {
	case "tar.gz":
		return unpackTarGz(data)
	case "zip":
		return unpackZip(data)
	}
	return nil, fmt.Errorf("unknown archive format %q", format)
}

func unpackTarGz(data []byte) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()

	files := make(map[string][]byte)
	var total int64
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			// symlinks, devices, and hard links have no business in a suite
			// of text files
			return nil, fmt.Errorf("%s: unsupported archive member type %q", hdr.Name, hdr.Typeflag)
		}
		if err := safeName(hdr.Name); err != nil {
			return nil, err
		}
		if hdr.Size > maxMember {
			return nil, fmt.Errorf("%s: member is %d bytes, over the %d byte limit", hdr.Name, hdr.Size, maxMember)
		}
		b, err := io.ReadAll(io.LimitReader(tr, maxMember+1))
		if err != nil {
			return nil, err
		}
		if int64(len(b)) > maxMember {
			return nil, fmt.Errorf("%s: member is over the %d byte limit", hdr.Name, maxMember)
		}
		total += int64(len(b))
		if total > maxTotal {
			return nil, fmt.Errorf("archive expands past the %d byte limit", maxTotal)
		}
		if _, dup := files[hdr.Name]; dup {
			return nil, fmt.Errorf("%s: appears twice in the archive", hdr.Name)
		}
		files[hdr.Name] = b
	}
	return files, nil
}

func unpackZip(data []byte) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	files := make(map[string][]byte)
	var total int64
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue // a directory entry
		}
		if !f.Mode().IsRegular() {
			return nil, fmt.Errorf("%s: not a regular file", f.Name)
		}
		if err := safeName(f.Name); err != nil {
			return nil, err
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, maxMember+1))
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		if int64(len(b)) > maxMember {
			return nil, fmt.Errorf("%s: member is over the %d byte limit", f.Name, maxMember)
		}
		total += int64(len(b))
		if total > maxTotal {
			return nil, fmt.Errorf("archive expands past the %d byte limit", maxTotal)
		}
		if _, dup := files[f.Name]; dup {
			return nil, fmt.Errorf("%s: appears twice in the archive", f.Name)
		}
		files[f.Name] = b
	}
	return files, nil
}

// verify compares what came out of an archive against what the manifest says
// should be there, reporting everything that is wrong rather than the first
// thing.
func verify(a *Archive, files map[string][]byte) error {
	var problems []string
	listed := make(map[string]bool, len(a.Files))
	for _, want := range a.Files {
		listed[want.Name] = true
		got, ok := files[want.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf("  %s: missing", want.Name))
			continue
		}
		if want.Size != 0 && int64(len(got)) != want.Size {
			problems = append(problems, fmt.Sprintf("  %s: %d bytes, want %d", want.Name, len(got), want.Size))
			continue
		}
		if sum := digest(got); sum != want.SHA256 {
			problems = append(problems, fmt.Sprintf("  %s: sha256 %s, want %s", want.Name, sum, want.SHA256))
		}
	}
	for name := range files {
		if !listed[name] {
			problems = append(problems, fmt.Sprintf("  %s: not in the manifest", name))
		}
	}
	if len(problems) != 0 {
		sortStrings(problems)
		return fmt.Errorf("%s does not match the manifest:\n%s", a.Name, strings.Join(problems, "\n"))
	}
	return nil
}

// sortStrings is a small insertion sort; the lists here are tens of entries
// and this keeps the reporting deterministic without pulling in a dependency.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
