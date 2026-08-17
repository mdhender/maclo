// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package stmt

import "testing"

// TestTableIndexMatchesKind is the test that lets there be one list instead of
// three. pkg/lowl/op keeps an enum, a String switch and a Lookup map in step by
// hand; here String and Lookup both read table, and the only thing that can go
// wrong is an entry landing at the wrong index. So check that, and the whole
// class of drift the LOWL packages are exposed to cannot happen.
func TestTableIndexMatchesKind(t *testing.T) {
	for i := Unknown; i < numKinds; i++ {
		e := table[i]
		if e.Kind != i {
			t.Errorf("table[%d]: Kind is %d (%q): every entry must sit at its own index", int(i), int(e.Kind), e.Kind)
		}
		if i == Unknown {
			continue
		}
		if len(e.Words) == 0 {
			t.Errorf("table[%d]: no spelling: the entry is a hole in the enum", int(i))
		}
		if e.Cat == CatNone {
			t.Errorf("%s: no category", i)
		}
		if e.Where == 0 {
			t.Errorf("%s: no sections: sema cannot say where it may appear", i)
		}
		if e.Doc == "" {
			t.Errorf("%s: no citation in the L manual", i)
		}
	}
}

// TestSpellingsAreUnique guards the other half of a table-driven Lookup: two
// entries with one spelling would make the map silently prefer whichever was
// built last.
func TestSpellingsAreUnique(t *testing.T) {
	seen := map[string]Kind{}
	for i := Unknown + 1; i < numKinds; i++ {
		s := i.String()
		if prev, ok := seen[s]; ok {
			t.Errorf("%q is the spelling of both %d and %d", s, int(prev), int(i))
		}
		seen[s] = i
	}
}

// TestWordCount holds the assumption Lookup is built on. Every multi-word head
// in L is exactly two words, so the longest match is a two-step. A three-word
// head would make Lookup quietly wrong rather than fail to compile.
func TestWordCount(t *testing.T) {
	for i := Unknown + 1; i < numKinds; i++ {
		if n := len(table[i].Words); n != 1 && n != 2 {
			t.Errorf("%s: %d words: Lookup only tries two then one", i, n)
		}
	}
}

func TestLookup(t *testing.T) {
	for _, tc := range []struct {
		name     string
		words    []string
		want     Kind
		consumed int
		ok       bool
	}{
		{"one word", []string{"SET", "A", "=", "0"}, Set, 1, true},
		{"two words", []string{"GO", "TO", "MBEGIN"}, GoTo, 2, true},
		{"two words wins over one", []string{"EXIT", "FROM", "CMPARE"}, ExitFrom, 2, true},
		{"chain from", []string{"CHAIN", "FROM", "DELPT"}, ChainFrom, 2, true},
		{"munstack from", []string{"MUNSTACK", "FROM"}, MUnstackFrom, 2, true},
		{"link back", []string{"LINK", "BACK"}, LinkBack, 2, true},
		{"return from", []string{"RETURN", "FROM", "ADVNCE"}, ReturnFrom, 2, true},
		{"no trailing word", []string{"ENDCH"}, EndCh, 1, true},
		{"not a head", []string{"THEN"}, Unknown, 0, false},
		{"type suffix is not a head", []string{"PT"}, Unknown, 0, false},
		{"macro is not a head", []string{"IND"}, Unknown, 0, false},
		{"first of two alone", []string{"GO"}, Unknown, 0, false},
		{"first of two with a stranger", []string{"GO", "BACK"}, Unknown, 0, false},
		{"nothing", nil, Unknown, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, consumed, ok := Lookup(tc.words...)
			if got != tc.want || consumed != tc.consumed || ok != tc.ok {
				t.Errorf("Lookup(%q) = (%s, %d, %v), want (%s, %d, %v)",
					tc.words, got, consumed, ok, tc.want, tc.consumed, tc.ok)
			}
		})
	}
}

// TestEXITisNotAStatementHeadAlone is the reason the enum holds statement heads
// only. EXIT is a keyword of SUBROUTINE (lmap.txt 4.1.1.1) and the first word
// of EXIT FROM (lmap.txt 4.1.1.3). Keeping the keyword out of the table is what
// lets Lookup be called at the head position without a special case.
func TestEXITisNotAStatementHeadAlone(t *testing.T) {
	if k, _, ok := Lookup("EXIT"); ok {
		t.Errorf("Lookup(EXIT) resolved to %s: EXIT alone is a keyword of SUBROUTINE, not a statement", k)
	}
	if k, n, ok := Lookup("EXIT", "FROM"); !ok || k != ExitFrom || n != 2 {
		t.Errorf("Lookup(EXIT, FROM) = (%s, %d, %v), want (EXIT FROM, 2, true)", k, n, ok)
	}
}

// TestClosers pins the pairing table, and in particular the one many-to-one
// case: LINKROUTINE has no closer of its own and ends with ENDSUB.
func TestClosers(t *testing.T) {
	for _, tc := range []struct {
		closer  Kind
		openers []Kind
	}{
		{EndSect, []Kind{Section}},
		{EndBlock, []Kind{BlockDec}},
		{EndSub, []Kind{Subroutine, LinkRoutine}},
		{End, []Kind{If}},
		{EndCh, []Kind{ChainFrom}},
	} {
		t.Run(tc.closer.String(), func(t *testing.T) {
			if tc.closer.Role() != RoleClose {
				t.Errorf("%s: role is %s, want close", tc.closer, tc.closer.Role())
			}
			for _, o := range tc.openers {
				if !tc.closer.Closes(o) {
					t.Errorf("%s does not close %s", tc.closer, o)
				}
				if o.Role() != RoleOpen {
					t.Errorf("%s: role is %s, want open", o, o.Role())
				}
			}
			if tc.closer.Closes(Set) {
				t.Errorf("%s claims to close SET", tc.closer)
			}
		})
	}
	// Every opener must have exactly one closer that admits it, and every
	// closer must be in the map. Otherwise build.go has a construct it cannot
	// finish, or one it cannot start.
	for i := Unknown + 1; i < numKinds; i++ {
		switch i.Role() {
		case RoleClose:
			if len(i.OpenedBy()) == 0 {
				t.Errorf("%s closes nothing", i)
			}
		case RoleOpen:
			var closers int
			for j := Unknown + 1; j < numKinds; j++ {
				if j.Closes(i) {
					closers++
				}
			}
			if closers != 1 {
				t.Errorf("%s has %d closers, want 1", i, closers)
			}
		}
	}
}

// TestSectionRules spot-checks the placement masks sema reads instead of
// writing a switch of its own.
func TestSectionRules(t *testing.T) {
	for _, tc := range []struct {
		kind Kind
		want Sections
	}{
		{PrgStart, InFrame},
		{Section, InFrame},
		{Dec, InVARS},
		{BlockDec, InVARS},
		{Set, InProgram},
		{ChainFrom, InProgram},
		{DC, InData},
		{OpMac, InData},
		{HETables, InData},
	} {
		if got := tc.kind.Sections(); got != tc.want {
			t.Errorf("%s: sections %s, want %s", tc.kind, got, tc.want)
		}
	}
}
