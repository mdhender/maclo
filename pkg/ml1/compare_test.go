// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1_test

import (
	"strings"
	"testing"
)

// compareFunc reports whether got matches want and, when it does not, locates
// the first line that differs. The line number is one based; the two strings
// are the offending lines, or a marker when one side ran out first.
//
// There are two implementations because the two golden corpora are held to
// different standards. See equalExact and equalIgnoringSpaceChange.
type compareFunc func(got, want string) (ok bool, line int, gotLine, wantLine string)

const (
	endOfOutput = "<end of output>"
	endOfGolden = "<end of golden>"
)

// equalExact compares byte for byte. Nothing is normalized: not line endings,
// not trailing blanks, not the final newline.
//
// This is what the corpus in this repository is held to. We write both sides
// of that comparison, so there is no reason to inherit the tolerance the
// upstream harness needs, and in a macro processor a stray space is a real
// defect rather than a formatting quibble. It is safe to be this strict only
// because .gitattributes pins the golden files to eol=lf, so a checkout on
// any platform produces the same bytes.
func equalExact(got, want string) (bool, int, string, string) {
	if got == want {
		return true, 0, "", ""
	}
	// splitting without trimming keeps a missing final newline visible: it
	// makes one side one element shorter
	line, gotLine, wantLine := locate(strings.Split(got, "\n"), strings.Split(want, "\n"))
	return false, line, gotLine, wantLine
}

// equalIgnoringSpaceChange compares the way diff(1) does when it is given -b,
// which is what the upstream runtest.sh uses:
//
//	../ml1 -v sets18.ml1 $1.ml1 -o $1.tmp_out -d $1.tmp_err
//	diff -b $1.out $1.tmp_out
//
// Matching that is a conformance obligation, not a preference, so the rules
// are theirs and were checked against the real diff rather than recalled:
//
//   - whitespace at the end of a line is ignored, so a line of nothing but
//     blanks compares equal to an empty line;
//   - every other run of blanks compares equal to any other run, so one space
//     matches two, and a space matches a tab;
//   - a run at the start of a line collapses to one blank rather than
//     vanishing, so "foo" and " foo" still differ. This is the part that is
//     easy to get backwards; -w is the flag that deletes whitespace outright;
//   - a blank line is still a line, so an inserted or deleted one differs;
//   - a missing final newline is not a difference.
//
// Carriage returns are folded into the trailing trim, which is not part of -b
// but lets a golden file with DOS line endings compare against output with
// Unix ones.
//
// These rules were checked differentially against /usr/bin/diff -b over 20000
// randomly generated pairs built from blank-heavy lines: no disagreements.
//
// There is one known divergence, and it is deliberately not chased. When a
// file's last line has no terminating newline *and* carries trailing blanks,
// BSD diff stops ignoring those blanks, so it calls "a " and "a\n" different
// even though it calls "a" and "a\n" the same. The behaviour looks like an
// artefact rather than a rule, GNU diff need not share it, and it cannot come
// up here: every one of the 22 upstream golden files ends with a newline. We
// treat a missing final newline as insignificant in all cases, which
// TestCompare pins.
func equalIgnoringSpaceChange(got, want string) (bool, int, string, string) {
	g, w := normalizeLines(got), normalizeLines(want)
	if len(g) == len(w) {
		same := true
		for i := range g {
			if g[i] != w[i] {
				same = false
				break
			}
		}
		if same {
			return true, 0, "", ""
		}
	}
	line, gotLine, wantLine := locate(g, w)
	return false, line, gotLine, wantLine
}

// locate returns the first line at which two already-comparable slices
// diverge, along with both sides.
func locate(g, w []string) (int, string, string) {
	for i := 0; i < len(g) || i < len(w); i++ {
		gl, wl := endOfOutput, endOfGolden
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl {
			return i + 1, gl, wl
		}
	}
	return 0, "", ""
}

// normalizeLines applies the diff -b rules to every line of s. A trailing
// newline terminates the last line rather than starting an empty one.
func normalizeLines(s string) []string {
	// an empty file has no lines at all, but a file holding just a newline
	// has one empty line, so this test has to come before the trim
	if s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "\n")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = squeezeBlanks(line)
	}
	return lines
}

// squeezeBlanks drops the run of blanks at the end of a line and collapses
// every other run to a single space. It works a byte at a time and leaves
// bytes above 0x7f alone, so it is safe on UTF-8 without decoding it.
func squeezeBlanks(line string) string {
	line = strings.TrimRight(line, " \t\r\v\f")
	var b strings.Builder
	b.Grow(len(line))
	pending := false
	for i := 0; i < len(line); i++ {
		switch c := line[i]; c {
		case ' ', '\t', '\r', '\v', '\f':
			pending = true
		default:
			if pending {
				b.WriteByte(' ')
				pending = false
			}
			b.WriteByte(c)
		}
	}
	return b.String()
}

// TestCompare pins both comparators against the same table.
//
// Every case records what each one is supposed to say. The rows where they
// disagree are the point of the test: if equalExact ever grew line-ending or
// trailing-whitespace tolerance, the corpus in this repository would quietly
// stop being the stricter of the two and nobody would notice.
func TestCompare(t *testing.T) {
	for _, tc := range []struct {
		id        int
		name      string
		got, want string
		loose     bool // what equalIgnoringSpaceChange should say
		strict    bool // what equalExact should say
	}{
		{1, "identical", "foo\n", "foo\n", true, true},
		{2, "both empty", "", "", true, true},
		{3, "trailing space added", "foo \n", "foo\n", true, false},
		{4, "trailing tab added", "foo\t\n", "foo\n", true, false},
		{5, "interior run widened", "a  b\n", "a b\n", true, false},
		{6, "interior tab for space", "a\tb\n", "a b\n", true, false},
		{7, "blank only line vs empty", "   \n", "\n", true, false},
		{8, "missing final newline", "foo", "foo\n", true, false},
		{9, "CRLF vs LF", "foo\r\n", "foo\n", true, false},
		{10, "leading space added", " foo\n", "foo\n", false, false},
		{11, "leading run widened", "  foo\n", " foo\n", true, false},
		{12, "interior space inserted", "a b\n", "ab\n", false, false},
		{13, "extra blank line", "a\n\nb\n", "a\nb\n", false, false},
		{14, "different text", "bar\n", "foo\n", false, false},
		{15, "extra line at end", "a\nb\n", "a\n", false, false},
		{16, "missing line at end", "a\n", "a\nb\n", false, false},
		// the documented divergence from BSD diff, pinned so that a change
		// to it is a deliberate act. The real diff -b calls this pair
		// different; we do not, and no golden file can reach the case.
		{17, "unterminated last line with trailing blank", "a ", "a\n", true, false},
	} {
		if ok, _, _, _ := equalIgnoringSpaceChange(tc.got, tc.want); ok != tc.loose {
			t.Errorf("%d: %s: diff -b: want %v: got %v\n", tc.id, tc.name, tc.loose, ok)
		}
		if ok, _, _, _ := equalExact(tc.got, tc.want); ok != tc.strict {
			t.Errorf("%d: %s: exact: want %v: got %v\n", tc.id, tc.name, tc.strict, ok)
		}
		// the strict comparator must never accept what the tolerant one
		// rejects; that would mean they had diverged in the wrong direction
		if tc.strict && !tc.loose {
			t.Errorf("%d: %s: exact accepts what diff -b rejects\n", tc.id, tc.name)
		}
	}
}

// TestCompareLocatesDifference checks the reporting, since the whole failure
// mode these comparators hide is a difference nobody can see on screen.
func TestCompareLocatesDifference(t *testing.T) {
	for _, tc := range []struct {
		id       int
		name     string
		equal    compareFunc
		got      string
		want     string
		line     int
		gotLine  string
		wantLine string
	}{
		{1, "exact finds the trailing blank", equalExact,
			"a\nb \nc\n", "a\nb\nc\n", 2, "b ", "b"},
		{2, "exact finds a missing final newline", equalExact,
			"a", "a\n", 2, endOfOutput, ""},
		{3, "diff -b finds the indent", equalIgnoringSpaceChange,
			"a\n b\n", "a\nb\n", 2, " b", "b"},
		{4, "diff -b finds a short output", equalIgnoringSpaceChange,
			"a\n", "a\nb\n", 2, endOfOutput, "b"},
		{5, "diff -b finds a long output", equalIgnoringSpaceChange,
			"a\nb\n", "a\n", 2, "b", endOfGolden},
	} {
		ok, line, gotLine, wantLine := tc.equal(tc.got, tc.want)
		if ok {
			t.Errorf("%d: %s: want a difference: got none\n", tc.id, tc.name)
			continue
		}
		if line != tc.line || gotLine != tc.gotLine || wantLine != tc.wantLine {
			t.Errorf("%d: %s: want %d %q %q: got %d %q %q\n",
				tc.id, tc.name, tc.line, tc.gotLine, tc.wantLine, line, gotLine, wantLine)
		}
	}
}
