// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package lmap_test

import (
	"fmt"

	"github.com/mdhender/maclo/pkg/lowl/assembler"
	"github.com/mdhender/maclo/pkg/lowl/ast"
	"github.com/mdhender/maclo/pkg/lowl/cst"
	"github.com/mdhender/maclo/pkg/lowl/vm"
)

// assemble is pkg/ml1's own three lines, repeated here because that one is not
// exported and because a test that reached for it would be testing the
// processor rather than the L-map.
//
// It is the strongest check this package has. Every RL distance, every exit
// number, every name and every operand shape is decided here rather than
// argued about, and the answer is a machine or a line number.
func assemble(source []byte) (*vm.VM, error) {
	nodes := cst.ParseBuffer(source)
	for _, node := range nodes {
		if node.Error != nil {
			return nil, fmt.Errorf("%d:%d: %w", node.Line, node.Col, node.Error)
		}
	}
	tree, err := ast.Parse(nodes)
	if err != nil {
		return nil, err
	}
	return assembler.Assemble(tree, assembler.Options{})
}
