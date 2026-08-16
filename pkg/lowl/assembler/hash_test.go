// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package assembler_test

import (
	"testing"

	"github.com/mdhender/maclo/pkg/lowl/assembler"
	"github.com/mdhender/maclo/pkg/lowl/ast"
	"github.com/mdhender/maclo/pkg/lowl/op"
	"github.com/mdhender/maclo/pkg/lowl/vm"
)

// The nodes are built here rather than assembled from a source file because
// the only real LOWL source of ML/I cannot be committed. A handful of nodes is
// enough: what is being tested is the shape of the table, not the parser.

func quoted(text string) *ast.Parameter {
	return &ast.Parameter{Kind: ast.QuotedText, Text: text}
}

func variable(text string) *ast.Parameter {
	return &ast.Parameter{Kind: ast.Variable, Text: text}
}

func node(line int, code op.Code, parms ...*ast.Parameter) *ast.Node {
	return &ast.Node{Line: line, Op: code, Parameters: parms}
}

// TestHashTable checks that HASH and THASH build a table the name search in
// ML/I can walk: every built-in macro is on the chain its name hashes to, it is
// on that chain exactly once, and every chain ends.
func TestHashTable(t *testing.T) {
	// names chosen to collide: with 20 or so built-ins and 32 chains, several
	// share a chain in the real source, and a table that only works when every
	// name lands in its own bucket would look fine right up until it did not.
	names := []string{"MCDEF", "MCSKIP", "MCINS", "MCWARN", "MCGO", "MCSET", "MCNOTE", "MCSUB"}

	var nodes ast.Nodes
	nodes = append(nodes, node(1, op.MDLABEL, &ast.Parameter{Kind: ast.Label, Text: "BEGIN"}))
	nodes = append(nodes, node(2, op.CSS))
	line := 3
	for _, name := range names {
		nodes = append(nodes, node(line, op.HASH, quoted(name)))
		line++
	}
	tableNode := line
	nodes = append(nodes, node(tableNode, op.THASH))
	nodes = append(nodes, node(line+1, op.PRGEN))

	m, err := assembler.Assemble(nodes, assembler.Options{})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	// find the words the statements laid down. The HASH words are in source
	// order, and THASH follows them.
	var entries []int
	head := -1
	for addr := 0; addr <= m.Registers.Last; addr++ {
		switch m.Core[addr].Op {
		case op.HASH:
			entries = append(entries, addr)
		case op.THASH:
			if head == -1 {
				head = addr
			}
		}
	}
	if len(entries) != len(names) {
		t.Fatalf("hash words: want %d: got %d", len(names), len(entries))
	}
	if head == -1 {
		t.Fatalf("thash: no chain heads emitted")
	}

	// walk every chain, and record where each entry was found.
	foundOn := make(map[int]int) // entry address -> chain head it hung from
	for bucket := 0; bucket < vm.LHV; bucket += m.Registers.LNM {
		seen := 0
		for at := m.Core[head+bucket].Value; at != 0; at = m.Core[at].Value {
			if m.Core[at].Op != op.HASH {
				t.Fatalf("chain %d: address %d holds %s, want HASH", bucket, at, m.Core[at].Op)
			}
			if where, ok := foundOn[at]; ok {
				t.Fatalf("chain %d: %q also on chain %d", bucket, m.Core[at].Text, where)
			}
			foundOn[at] = bucket
			// a chain cannot be longer than the table, so this catches a cycle
			// instead of hanging the test.
			if seen++; seen > len(names) {
				t.Fatalf("chain %d: does not end", bucket)
			}
		}
	}

	// every name must be reachable, and on the chain its own hash names.
	for i, addr := range entries {
		bucket, ok := foundOn[addr]
		if !ok {
			t.Errorf("%q: not on any chain", names[i])
			continue
		}
		if want := vm.HashName(names[i], m.Registers.LCH, m.Registers.LNM); bucket != want {
			t.Errorf("%q: on chain %d: want %d", names[i], bucket, want)
		}
	}
}

// TestHashTableRejects covers the two ways the table can be malformed. Neither
// can happen in the real source, and both would fail confusingly at run time.
func TestHashTableRejects(t *testing.T) {
	begin := node(1, op.MDLABEL, &ast.Parameter{Kind: ast.Label, Text: "BEGIN"})

	if _, err := assembler.Assemble(ast.Nodes{
		begin,
		node(2, op.HASH, quoted("MCDEF")),
		node(3, op.PRGEN),
	}, assembler.Options{}); err == nil {
		t.Errorf("hash without thash: want an error: got nil")
	}

	if _, err := assembler.Assemble(ast.Nodes{
		begin,
		node(2, op.HASH, quoted("MCDEF")),
		node(3, op.HASH, quoted("MCDEF")),
		node(4, op.THASH),
		node(5, op.PRGEN),
	}, assembler.Options{}); err == nil {
		t.Errorf("duplicate name: want an error: got nil")
	}
}

// TestRelativeLocation checks what RL stores.
//
// It is the distance from the word itself to the label, not the label's
// address, because ML/I walks its tables by adding the word it has just read
// to the address it read it from. Storing the address instead is not a wrong
// number in an obvious way: the tables still assemble and most of the
// processor still works, and what shows up much later is a delimiter printed
// as the middle of some other string.
func TestRelativeLocation(t *testing.T) {
	// three words of padding, so that the label is a known distance ahead of
	// the RL that names it
	m, err := assembler.Assemble(ast.Nodes{
		node(1, op.MDLABEL, &ast.Parameter{Kind: ast.Label, Text: "BEGIN"}),
		node(2, op.RL, variable("DVARS"),
			&ast.Parameter{Kind: ast.Macro, Text: "OF"},
			&ast.Parameter{Kind: ast.Expression, Text: "(1*LNM+3*LCH)"}),
		node(3, op.STR, &ast.Parameter{Kind: ast.QuotedText, Text: "abc"}),
		node(4, op.MDLABEL, &ast.Parameter{Kind: ast.Label, Text: "DVARS"}),
		node(5, op.CON, &ast.Parameter{Kind: ast.Number, Number: 0}),
		node(6, op.PRGEN),
	}, assembler.Options{})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	var item, target int
	for addr := 0; addr <= m.Registers.Last; addr++ {
		if m.Core[addr].Op == op.RL {
			item = addr
		} else if m.Core[addr].Op == op.CON {
			target = addr
		}
	}
	if item == 0 {
		t.Fatalf("rl: no table item emitted")
	}
	// LCH and LNM are both one word, so the label is 1+3 words on: the RL
	// itself and the three characters of the string.
	if want := target - item; m.Core[item].Value != want {
		t.Errorf("rl: want the distance %d (label %d, item %d): got %d", want, target, item, m.Core[item].Value)
	}
	if m.Core[item].Value != 4 {
		t.Errorf("rl: want the distance the source gave (4): got %d", m.Core[item].Value)
	}
}

// TestRelativeLocationChecked covers the other half: an RL whose offset does
// not match what was laid out is reported rather than stored. The offset is
// the source's own statement of the distance, so a disagreement means the
// table has been assembled into a different shape than it was written in.
func TestRelativeLocationChecked(t *testing.T) {
	_, err := assembler.Assemble(ast.Nodes{
		node(1, op.MDLABEL, &ast.Parameter{Kind: ast.Label, Text: "BEGIN"}),
		node(2, op.RL, variable("DVARS"),
			&ast.Parameter{Kind: ast.Macro, Text: "OF"},
			&ast.Parameter{Kind: ast.Expression, Text: "(9*LNM)"}),
		node(3, op.MDLABEL, &ast.Parameter{Kind: ast.Label, Text: "DVARS"}),
		node(4, op.CON, &ast.Parameter{Kind: ast.Number, Number: 0}),
		node(5, op.PRGEN),
	}, assembler.Options{})
	if err == nil {
		t.Errorf("rl: want an error for an offset that does not match: got nil")
	}
}
