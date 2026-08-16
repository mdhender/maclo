// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ast

import "github.com/maloquacious/ml_i/pkg/lowl/op"

type Node struct {
	Line, Col  int
	Op         op.Code
	Parameters Parameters
}
