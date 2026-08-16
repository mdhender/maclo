// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1

import (
	"bytes"
	"io"
	"testing"

	"github.com/maloquacious/ml_i/pkg/lowl/vm"
)

// The listing and the output line count, driven through MDOUCH directly.
//
// The golden corpus covers both end to end, but it skips on a machine that has
// not fetched the LOWL source of ML/I, and these two rules are the sort that go
// wrong quietly: a listing that is one line out is still a plausible listing.
// Nothing here needs the engine, so this coverage is always there.

// writeChars is a process that writes text and nothing else.
func writeChars(t *testing.T, h *host, text string) {
	t.Helper()
	for i := 0; i < len(text); i++ {
		if err := h.WriteChar(int(text[i])); err != nil {
			t.Fatalf("write %q: %v", text[i], err)
		}
	}
}

// newTestHost builds the storage a host needs and nothing else: the
// S-variables, in the block SVARPT points at, at their initial values.
func newTestHost(t *testing.T, job Job) *host {
	t.Helper()
	m := vm.New()
	m.Registers.Last = 400
	m.Symbols["SVARPT"] = 20
	if _, err := m.SetSystemVariables(0, systemVariables()); err != nil {
		t.Fatalf("svars: %v", err)
	}
	h := &host{job: job, m: m, atLineStart: true}
	h.svarpt = m.Core[20].Value
	return h
}

// TestListingIsControlledByS20 covers the three things S20 selects, including
// being changed part way through a process.
func TestListingIsControlledByS20(t *testing.T) {
	for _, tc := range []struct {
		name string
		s20  int
		want string
	}{
		{"off", 0, ""},
		{"plain", 1, "one\ntwo\n"},
		{"numbered", 2, "    1.   one\n    2.   two\n"},
		// zero is the only value that means anything but "list it"
		{"any other value lists without numbers", 7, "one\ntwo\n"},
		{"negative", -1, "one\ntwo\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, dbg, lst bytes.Buffer
			h := newTestHost(t, Job{Outputs: []io.Writer{&out}, Debug: &dbg, Listing: &lst})
			h.setSV(svListing, tc.s20)

			writeChars(t, h, "one\ntwo\n")

			if got := lst.String(); got != tc.want {
				t.Errorf("listing: want %q: got %q", tc.want, got)
			}
			// whatever S20 says, the output text is the same
			if got := out.String(); got != "one\ntwo\n" {
				t.Errorf("output: want %q: got %q", "one\ntwo\n", got)
			}
		})
	}
}

// TestListingCanBeSwitchedOffAndOn checks what a listing that was interrupted
// looks like. Nothing marks the gap, so the numbers are what say a line is
// missing, and they are of output lines rather than of listed ones.
func TestListingCanBeSwitchedOffAndOn(t *testing.T) {
	var out, dbg, lst bytes.Buffer
	h := newTestHost(t, Job{Outputs: []io.Writer{&out}, Debug: &dbg, Listing: &lst})

	h.setSV(svListing, listingNumbered)
	writeChars(t, h, "one\n")
	h.setSV(svListing, 0)
	writeChars(t, h, "two\nthree\n")
	h.setSV(svListing, listingNumbered)
	writeChars(t, h, "four\n")

	want := "    1.   one\n    4.   four\n"
	if got := lst.String(); got != want {
		t.Errorf("listing: want %q: got %q", want, got)
	}
}

// TestOutputLineCount pins where S19 is stepped.
//
// It is the number of the line being written, not the count of the ones
// finished, so it starts at zero and reaches one when the first character
// arrives. That is visible to a macro, which reads it between two lines, and it
// is the number the listing prints.
func TestOutputLineCount(t *testing.T) {
	var out, dbg bytes.Buffer
	h := newTestHost(t, Job{Outputs: []io.Writer{&out}, Debug: &dbg})

	if got := h.sv(svOutputLine); got != 0 {
		t.Errorf("before any output: want 0: got %d", got)
	}
	writeChars(t, h, "one")
	if got := h.sv(svOutputLine); got != 1 {
		t.Errorf("in the first line: want 1: got %d", got)
	}
	// the newline that ends a line does not step the count; the first
	// character of the next one does.
	writeChars(t, h, "\n")
	if got := h.sv(svOutputLine); got != 1 {
		t.Errorf("between two lines: want 1: got %d", got)
	}
	writeChars(t, h, "t")
	if got := h.sv(svOutputLine); got != 2 {
		t.Errorf("in the second line: want 2: got %d", got)
	}

	// an empty line is a line: the newline is its first character
	writeChars(t, h, "wo\n\n")
	if got := h.sv(svOutputLine); got != 3 {
		t.Errorf("after an empty line: want 3: got %d", got)
	}
}

// TestListingIgnoresTheOutputMask checks that S21 does not reach the listing.
// A masked line is still written as far as MDOUCH is concerned: it counts and
// it is listed, it just does not reach a file.
func TestListingIgnoresTheOutputMask(t *testing.T) {
	var out, dbg, lst bytes.Buffer
	h := newTestHost(t, Job{Outputs: []io.Writer{&out}, Debug: &dbg, Listing: &lst})
	h.setSV(svListing, listingNumbered)

	writeChars(t, h, "shown\n")
	h.setSV(svOutputMask, 0)
	writeChars(t, h, "hidden\n")
	h.setSV(svOutputMask, 1)
	writeChars(t, h, "shown again\n")

	if got, want := out.String(), "shown\nshown again\n"; got != want {
		t.Errorf("output: want %q: got %q", want, got)
	}
	want := "    1.   shown\n    2.   hidden\n    3.   shown again\n"
	if got := lst.String(); got != want {
		t.Errorf("listing: want %q: got %q", want, got)
	}
}

// TestNoListingAsked covers a job with nowhere to put a listing. S20 is the
// user's to set whether or not they gave -l, so asking for one that cannot be
// written is not an error.
func TestNoListingAsked(t *testing.T) {
	var out, dbg bytes.Buffer
	h := newTestHost(t, Job{Outputs: []io.Writer{&out}, Debug: &dbg})
	h.setSV(svListing, listingNumbered)

	writeChars(t, h, "one\ntwo\n")

	if got, want := out.String(), "one\ntwo\n"; got != want {
		t.Errorf("output: want %q: got %q", want, got)
	}
	if got := h.sv(svOutputLine); got != 2 {
		t.Errorf("line count: want 2: got %d", got)
	}
}
