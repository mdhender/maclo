// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

// Package ast accepts scanner tokens and returns an abstract syntax tree.
package ast

import (
	"fmt"
	"github.com/maloquacious/ml_i/pkg/lowl/cst"
	"github.com/maloquacious/ml_i/pkg/lowl/op"
)

type Nodes []*Node

func Parse(parseTree []*cst.Node) (Nodes, error) {
	var nodes Nodes
	for _, cnode := range parseTree {
		switch cnode.Kind {
		case cst.Error:
			return nil, cnode.Error
		case cst.Label:
			node := &Node{
				Line: cnode.Line,
				Col:  cnode.Col,
				Op:   op.MDLABEL,
				Parameters: []*Parameter{&Parameter{
					Line: cnode.Line,
					Col:  cnode.Col,
					Kind: Label,
					Text: cnode.String}}}
			nodes = append(nodes, node)
		case cst.OpCode:
			node := &Node{
				Line: cnode.Line,
				Col:  cnode.Col,
				Op:   cnode.OpCode}
			// add the parameters to the op-code
			for _, cparm := range cnode.Parameters {
				parm := &Parameter{Line: cparm.Line, Col: cparm.Col}
				switch cparm.Kind {
				case cst.Error:
					return nil, cparm.Error
				case cst.Expression:
					parm.Kind, parm.Text = Expression, cparm.String
				case cst.Macro:
					parm.Kind, parm.Text = Macro, cparm.String
				case cst.Number:
					parm.Kind, parm.Number = Number, cparm.Number
				case cst.QuotedText:
					parm.Kind, parm.Text = QuotedText, cparm.String
				case cst.Variable:
					parm.Kind, parm.Text = Variable, cparm.String
				default:
					panic(fmt.Sprintf("ast:%d:%d: unexpected %q\n", cnode.Line, cnode.Col, cnode.Kind))
				}
				node.Parameters = append(node.Parameters, parm)
			}
			nodes = append(nodes, node)
		default:
			return nil, fmt.Errorf("ast:%d:%d: unexpected %q\n", cnode.Line, cnode.Col, cnode.Kind)
		}
	}
	return nodes, nil
}
