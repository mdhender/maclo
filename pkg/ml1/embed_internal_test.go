// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ml1

import "testing"

// TestEngineVersion covers the parsing that decides what a binary calls the
// sources it carries.
//
// It is a separate test from the rest because it is the only part of the
// embedding that can be checked without an engine: a clone with an empty
// engines directory still runs this, which matters because the naming
// convention is what the ordering — and therefore the default engine —
// depends on.
func TestEngineVersion(t *testing.T) {
	for _, tc := range []struct {
		id   int
		name string
		want string
	}{
		{1, "ml1ajb", "AJB"},
		{2, "ml1aih", "AIH"},
		{3, "ML1AJB", "AJB"}, // the file system may not have preserved case
		{4, "ml1cqk", "CQK"},
		{5, "ml1", ""},      // no version letters at all
		{6, "ml1ab", ""},    // two, not three
		{7, "ml1abcd", ""},  // four
		{8, "ml1a1b", ""},   // a digit is not a version letter
		{9, "engine", ""},   // does not follow the convention
		{10, "", ""},        // and neither does nothing
		{11, "xml1ajb", ""}, // the prefix has to be at the start
	} {
		if got := engineVersion(tc.name); got != tc.want {
			t.Errorf("%d: engineVersion(%q): want %q: got %q", tc.id, tc.name, tc.want, got)
		}
	}
}

// TestReadSourceOrder pins which of the two Job fields is answered first
// without needing either an engine on disk or one in the binary: both names
// are wrong, and which error comes back says which was tried.
func TestReadSourceOrder(t *testing.T) {
	_, err := readSource("no-such-file.lwl", "ml1zzz")
	if err == nil {
		t.Fatalf("want an error, got nil")
	}
	if got := err.Error(); got[:len("no-such-file.lwl")] != "no-such-file.lwl" {
		t.Errorf("want the path to be tried first, got %v", err)
	}

	if _, err := readSource("", "ml1zzz"); err == nil {
		t.Errorf("an engine that is not embedded: want an error, got nil")
	}
}
