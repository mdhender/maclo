// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ast

import (
	"bytes"
	"fmt"
	"github.com/maloquacious/ml_i/pkg/lowl/op"
	"os"
)

func (nodes Nodes) Listing(name string) error {
	bb := &bytes.Buffer{}
	for _, node := range nodes {
		blabel := ""
		if node.Op == op.MDLABEL {
			blabel = node.Parameters.String() + ":"
		}
		_, _ = fmt.Fprintf(bb, "%-12s %-12s %-55s ;; %4d\n", blabel, node.Op, node.Parameters.String(), node.Line)
	}
	return os.WriteFile(name, bb.Bytes(), 0644)
}
