// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package scanner

import "github.com/maloquacious/ml_i/pkg/lowl/op"

func isOpCode(s string) (op.Code, bool) {
	code, ok := op.Lookup(s)
	return code, ok
}

func isMacro(s string) bool {
	return s == "OF"
}
