// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package postfix_test

import (
	"fmt"
	"github.com/maloquacious/ml_i/pkg/postfix"
	"testing"
)

func TestFromInfix(t *testing.T) {
	for _, tc := range []struct {
		id             int
		infix, postfix []string
	}{
		{id: 1,
			infix:   []string{"3", "-", "2"},
			postfix: []string{"3", "2", "-"}},
		{id: 2,
			infix:   []string{"A", "*", "B", "+", "C"},
			postfix: []string{"A", "B", "*", "C", "+"}},
		{id: 3,
			infix:   []string{"(", "A", "+", "B", ")", "*", "(", "C", "/", "D", ")"},
			postfix: []string{"A", "B", "+", "C", "D", "/", "*"}},
		{id: 4,
			infix:   []string{"A", "*", "(", "B", "*", "C", "+", "D", "*", "E", ")", "+", "F"},
			postfix: []string{"A", "B", "C", "*", "D", "E", "*", "+", "*", "F", "+"}},
		{id: 5,
			infix:   []string{"(", "A", "+", "B", ")", "*", "C", "+", "(", "D", "-", "E", ")", "/", "F", "+", "G"},
			postfix: []string{"A", "B", "+", "C", "*", "D", "E", "-", "F", "/", "+", "G", "+"}},
		{id: 6,
			infix:   []string{"A", "*", "(", "B", "*", "C", "+", "D", "*", "E", ")", "+", "F"},
			postfix: []string{"A", "B", "C", "*", "D", "E", "*", "+", "*", "F", "+"}},
	} {
		got := postfix.FromInfix(tc.infix)
		if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", tc.postfix) {
			t.Errorf("%d: want %v: got %v\n", tc.id, tc.postfix, got)
		}
	}
}
