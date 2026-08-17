// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package token

import "fmt"

// Position is a place in the source. Line and Col are 1-based; the zero
// Position means "nowhere", which is what a predefined symbol has.
type Position struct {
	Line int
	Col  int
}

// IsValid reports whether the position names a place in a file.
func (p Position) IsValid() bool { return p.Line > 0 }

// String implements the Stringer interface.
func (p Position) String() string {
	if !p.IsValid() {
		return "-"
	}
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

// Before reports whether p comes earlier in the source than q.
func (p Position) Before(q Position) bool {
	if p.Line != q.Line {
		return p.Line < q.Line
	}
	return p.Col < q.Col
}
