// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm

import (
	"fmt"
	"io"
)

// directLoad returns the value of variable v
func (m *VM) directLoad(v int) int {
	return m.Core[v].Value
}

// directStore saves the value into variable v
func (m *VM) directStore(v, value int) {
	m.Core[v].Value = value
}

// indexedLoad returns the contents of the address pointed to by B + n
func (m *VM) indexedLoad(n int) int {
	return m.Core[m.B+n].Value
}

// indirectLoad returns the contents of the address pointed to by V
func (m *VM) indirectLoad(v int) int {
	return m.Core[m.Core[v].Value].Value
}

// indirectStore saves the value into the address pointed to by v
func (m *VM) indirectStore(v, value int) {
	m.Core[m.Core[v].Value].Value = value
}

func printf(w io.Writer, format string, args ...any) {
	if w != nil {
		_, _ = fmt.Fprintf(w, format, args...)
	}
}

func isalpha(ch int) bool {
	return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z')
}
func isdigit(ch int) bool {
	return '0' <= ch && ch <= '9'
}

// ispunct is what GOPC branches on. The kernel manual defines a punctuation
// character as one that is not a letter and not a digit, but ML/I lets the user
// name one more character to be counted as alphanumeric, so the test is the
// machine's rather than a free function. See pseudoAlpha.
func (m *VM) ispunct(ch int) bool {
	if ch == m.pseudoAlpha() {
		return false
	}
	return !(isalpha(ch) || isdigit(ch))
}
