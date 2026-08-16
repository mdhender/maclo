// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

// Package cst returns a concrete syntax tree
package cst

import (
	"fmt"
	"github.com/mdhender/maclo/pkg/lowl/scanner"
)

func Parse(name string, testBuffer, testScanner bool) ([]*Node, error) {
	s, err := scanner.NewScanner(name)
	if err != nil {
		return nil, err
	}

	if testBuffer {
		return nil, s.TestBuffer()
	}

	if testScanner {
		return nil, s.TestScanner()
	}

	return parse(s), nil
}

// ParseBuffer parses source that is already in memory. It is the entry point
// for a library caller, which has no file to name and must not write the
// scanner's debug artifacts to whatever directory it happens to be run from.
func ParseBuffer(input []byte) []*Node {
	return parse(scanner.NewScannerFromBytes(input))
}

func parse(s *scanner.Scanner) []*Node {
	var nodes []*Node
	var node *Node
	for _, tok := range s.Tokens() {
		if tok.Kind == scanner.Unknown {
			if node != nil { // append the current node if it is valid
				nodes = append(nodes, node)
				node = nil
			}
			nodes = append(nodes, &Node{
				Line:  tok.Line,
				Col:   tok.Col,
				Kind:  Error,
				Error: fmt.Errorf("unknown token")})
		} else if tok.Kind == scanner.Error {
			if node != nil { // append the current node if it is valid
				nodes = append(nodes, node)
				node = nil
			}
			nodes = append(nodes, &Node{
				Line:  tok.Line,
				Col:   tok.Col,
				Kind:  Error,
				Error: tok.Value.Error})

		} else if tok.Kind == scanner.EndOfLine || tok.Kind == scanner.EndOfInput {
			if node != nil { // append the current node if it is valid
				nodes = append(nodes, node)
			}
			if tok.Kind == scanner.EndOfInput { // exit on end of input
				break
			}
			// create a new (blank) node for the next instruction
			node = nil
		} else if tok.Kind == scanner.Label {
			if node != nil {
				node.Error = fmt.Errorf("missing new-line?")
				nodes = append(nodes, node)
			}
			nodes = append(nodes, &Node{
				Line:   tok.Line,
				Col:    tok.Col,
				Kind:   Label,
				String: tok.Value.Label})
			node = nil
			continue
		} else if tok.Kind == scanner.OpCode {
			if node != nil {
				node.Error = fmt.Errorf("missing new-line?")
				nodes = append(nodes, node)
			}
			node = &Node{
				Line:   tok.Line,
				Col:    tok.Col,
				Kind:   OpCode,
				OpCode: tok.Value.OpCode}
			continue
		} else { // must be a parameter
			if node == nil {
				nodes = append(nodes, &Node{
					Line:  tok.Line,
					Col:   tok.Col,
					Kind:  Error,
					Error: fmt.Errorf("unexpected parameter %q", tok.String())})
				continue
			}
			switch tok.Kind {
			case scanner.Comma:
				// todo: fix this hack
				// hack - ignore commas
			case scanner.Expression:
				node.Parameters = append(node.Parameters, &Node{
					Line:   tok.Line,
					Col:    tok.Col,
					Kind:   Expression,
					String: tok.Value.Expression})
			case scanner.Macro:
				node.Parameters = append(node.Parameters, &Node{
					Line:   tok.Line,
					Col:    tok.Col,
					Kind:   Macro,
					String: tok.Value.Macro})
			case scanner.Number:
				node.Parameters = append(node.Parameters, &Node{
					Line:   tok.Line,
					Col:    tok.Col,
					Kind:   Number,
					Number: tok.Value.Number})
			case scanner.QuotedText:
				node.Parameters = append(node.Parameters, &Node{
					Line:   tok.Line,
					Col:    tok.Col,
					Kind:   QuotedText,
					String: tok.Value.QuotedText})
			case scanner.Text:
				node.Parameters = append(node.Parameters, &Node{
					Line:   tok.Line,
					Col:    tok.Col,
					Kind:   Text,
					String: tok.Value.Text})
			case scanner.Variable:
				node.Parameters = append(node.Parameters, &Node{
					Line:   tok.Line,
					Col:    tok.Col,
					Kind:   Variable,
					String: tok.Value.Variable})
			default:
				panic(fmt.Sprintf("unexpected parameter %q", tok.Kind))
			}
		}
	}
	return nodes
}
