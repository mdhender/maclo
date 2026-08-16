// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The golden files in testdata/local were produced by the reference
// implementation, so which build produced them is part of what they mean. ML/I
// is a family of ports and they do not all agree byte for byte, so a golden
// file generated with a different one would look fine and quietly encode
// different behaviour.
//
// These are the identifying facts, also recorded in docs/reference/golden-tests.md:
// ML/I on Apple (Intel) under macOS, implementation version 4.13, ML/I version
// CKQ, by Bob Eager, from https://www.ml1.org.uk/impl-ac.html.
const (
	oraclePath    = ".downloads/ml1"
	oracleVersion = "macOS version 4.13 (CKQ)"
	oracleSHA256  = "4ab419fafe8ecdcfd26c9701f7d15f74bb0a00deca4579ee8009c95601843fae"
)

// TestOracleIdentity checks the reference implementation when it is present.
//
// It is not required: the binary may not be redistributed, so it is gitignored
// and most checkouts will not have it, and nothing about running the tests
// depends on it. What this catches is the case that would otherwise be silent
// — someone has a *different* ML/I on hand and regenerates a golden file with
// it.
func TestOracleIdentity(t *testing.T) {
	path := filepath.Join(moduleRoot(), oraclePath)
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		t.Skipf("no reference implementation at %s; it is only needed to add a golden file", oraclePath)
	} else if err != nil {
		t.Fatalf("%s: %v\n", oraclePath, err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v\n", oraclePath, err)
	}
	sum := sha256.Sum256(b)
	if got := hex.EncodeToString(sum[:]); got != oracleSHA256 {
		t.Errorf("%s: not the build the golden files came from\n\twant sha256 %s\n\t got sha256 %s\n"+
			"\tsee docs/how-to/fetch-the-upstream-sources.md\n", oraclePath, oracleSHA256, got)
	}

	// -v reports on the standard error and does not stop the run, so it is
	// given no input to read
	cmd := exec.Command(path, "-v")
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s -v: %v\n", oraclePath, err)
	}
	if !strings.Contains(string(out), oracleVersion) {
		t.Errorf("%s -v: want %q, got %q\n", oraclePath, oracleVersion, strings.TrimSpace(string(out)))
	}
}
