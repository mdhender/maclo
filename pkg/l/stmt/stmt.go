// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package stmt

import (
	"fmt"
	"strings"
	"sync"
)

// String implements the Stringer interface. It reads the table rather than a
// switch of its own, which is the whole reason the table exists.
func (k Kind) String() string {
	if k == Unknown {
		return "unknown"
	}
	if !k.IsValid() {
		return fmt.Sprintf("Kind(%d)", int(k))
	}
	return strings.Join(table[k].Words, " ")
}

// IsValid reports whether k names a statement.
func (k Kind) IsValid() bool { return k > Unknown && k < numKinds }

// Category returns the manual's chapter grouping for k.
func (k Kind) Category() Category {
	if !k.IsValid() {
		return CatNone
	}
	return table[k].Cat
}

// Sections returns the set of SECTION classes k may appear in.
func (k Kind) Sections() Sections {
	if !k.IsValid() {
		return 0
	}
	return table[k].Where
}

// Role reports whether k opens a nested construct, closes one, or neither.
func (k Kind) Role() Role {
	if !k.IsValid() {
		return RolePlain
	}
	return table[k].Role
}

// Doc returns the citation in the L manual for k.
func (k Kind) Doc() string {
	if !k.IsValid() {
		return ""
	}
	return table[k].Doc
}

// Words returns the spelling of k as its separate words. The result aliases
// the table and must not be modified.
func (k Kind) Words() []string {
	if !k.IsValid() {
		return nil
	}
	return table[k].Words
}

// Closes reports whether k is a closer for opener. It is false for anything
// that is not a closer.
func (k Kind) Closes(opener Kind) bool {
	for _, o := range openedBy[k] {
		if o == opener {
			return true
		}
	}
	return false
}

// OpenedBy returns the statements k may close, or nil when k is not a closer.
func (k Kind) OpenedBy() []Kind { return openedBy[k] }

var (
	lookupOnce sync.Once
	oneWord    map[string]Kind
	twoWord    map[string]Kind
)

func buildLookup() {
	oneWord, twoWord = make(map[string]Kind), make(map[string]Kind)
	for k := Unknown + 1; k < numKinds; k++ {
		switch e := table[k]; len(e.Words) {
		case 1:
			oneWord[e.Words[0]] = k
		case 2:
			twoWord[e.Words[0]+" "+e.Words[1]] = k
		}
	}
}

// Lookup matches a statement head against the words at the front of a line and
// reports how many of them it consumed.
//
// It tries two words before one, which is what makes GO TO a statement rather
// than a GO with a stray TO. Every multi-word head in L is exactly two words,
// so the longest match never needs a third step.
func Lookup(words ...string) (Kind, int, bool) {
	lookupOnce.Do(buildLookup)
	if len(words) >= 2 {
		if k, ok := twoWord[words[0]+" "+words[1]]; ok {
			return k, 2, true
		}
	}
	if len(words) >= 1 {
		if k, ok := oneWord[words[0]]; ok {
			return k, 1, true
		}
	}
	return Unknown, 0, false
}

// String implements the Stringer interface.
func (s Sections) String() string {
	if s == 0 {
		return "nowhere"
	}
	var parts []string
	if s&InFrame != 0 {
		parts = append(parts, "frame")
	}
	if s&InVARS != 0 {
		parts = append(parts, "VARS")
	}
	if s&InProgram != 0 {
		parts = append(parts, "program")
	}
	if s&InData != 0 {
		parts = append(parts, "data")
	}
	return strings.Join(parts, "|")
}

// String implements the Stringer interface.
func (r Role) String() string {
	switch r {
	case RolePlain:
		return "plain"
	case RoleOpen:
		return "open"
	case RoleClose:
		return "close"
	}
	return "role"
}
