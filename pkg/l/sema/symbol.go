// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

// Package sema checks an L program: the structural rules that need no symbol
// table, and then the names.
//
// It resolves and does not type check. Every variable's type is recorded from
// the last two characters of its name (lmap.txt 3.2), because that costs
// nothing and is what a later pass would need, but nothing here enforces the
// manual's form-type rules. Those rules are subtler than the suffix - a
// constant may appear only after a relational operator, and an indirect
// address after one is always through a pointer - and a half-done type check
// is worse than none.
package sema

import (
	"fmt"

	"github.com/mdhender/maclo/pkg/l/ast"
	"github.com/mdhender/maclo/pkg/l/token"
)

// Kind is what a name turned out to be.
type Kind uint8

// enums for Kind
const (
	// Undefined is a name that was used and never declared. The symbol exists
	// so that every use of it can be gathered and reported once.
	Undefined Kind = iota
	Variable
	ProgLabel
	DataLabel
	SubName
	LinkName
	BlockName
	SectionName
	// Constant is a name the language defines: TRUE, NULLPT, STOPCODE, the
	// markers, the length macros, and the size of each declared block.
	Constant
	// MDLabel and MDSub are the machine-dependent logic, which is described in
	// the manual rather than written in L (lmap.txt 7).
	MDLabel
	MDSub
)

// String implements the Stringer interface.
func (k Kind) String() string {
	switch k {
	case Undefined:
		return "UNDEFINED"
	case Variable:
		return "variable"
	case ProgLabel:
		return "label"
	case DataLabel:
		return "data-label"
	case SubName:
		return "subroutine"
	case LinkName:
		return "linkroutine"
	case BlockName:
		return "block"
	case SectionName:
		return "section"
	case Constant:
		return "constant"
	case MDLabel:
		return "md-label"
	case MDSub:
		return "md-subroutine"
	}
	return "?"
}

// Use is the position a name was used in. It is what turns "this name is not a
// label" into a diagnostic: the check is a pair of Use and Kind.
type Use uint8

// enums for Use
const (
	AsValue Use = iota
	AsBranchTarget
	AsCallee
	AsAddress
	AsBlock
	AsReturnFrom
	AsExitFrom
	AsEndName
	AsRLTarget
	AsEquateSource
)

// String implements the Stringer interface.
func (u Use) String() string {
	switch u {
	case AsValue:
		return "a value"
	case AsBranchTarget:
		return "a branch target"
	case AsCallee:
		return "the subroutine of a CALL"
	case AsAddress:
		return "the argument of AD"
	case AsBlock:
		return "the argument of BLOCK"
	case AsReturnFrom:
		return "the subroutine of a RETURN FROM"
	case AsExitFrom:
		return "the subroutine of an EXIT FROM"
	case AsEndName:
		return "the name on a closing statement"
	case AsRLTarget:
		return "the argument of RL"
	case AsEquateSource:
		return "the variable an EQUATE stands for"
	}
	return "?"
}

// Reference is one use of a name.
type Reference struct {
	Pos token.Position
	Use Use
}

// Symbol is one name and everything known about it.
type Symbol struct {
	Name string
	Kind Kind
	// Def is where the name was declared. It is the zero Position for a
	// predefined name and for one that was never declared at all.
	Def        token.Position
	Predefined bool

	// Section is the SECTION the name was declared in, and Block the
	// enclosing BLOCKDEC for a variable. Both are empty when there is none.
	Section string
	Block   string

	// Type is inferred from the name and recorded, not enforced.
	Type ast.DataType

	// Param and HasExit describe a subroutine, and are what the CALL agreement
	// check compares against.
	Param   *ast.ParamSpec
	HasExit bool

	Refs []Reference
}

// IsDefined reports whether anything declared the name.
func (s *Symbol) IsDefined() bool { return s.Kind != Undefined }

// String implements the Stringer interface.
func (s *Symbol) String() string {
	return fmt.Sprintf("%s (%s)", s.Name, s.Kind)
}

// Table is the result of resolution.
type Table struct {
	byName map[string]*Symbol
	order  []*Symbol

	// Defs maps a defining occurrence to its symbol, and Uses a referencing
	// one. They live here rather than as pointers in the tree, the way
	// go/types keeps Info.Defs and Info.Uses out of go/ast, so that a tree
	// means the same thing whether or not a resolver has run.
	Defs map[ast.Node]*Symbol
	Uses map[ast.Node]*Symbol
}

// NewTable returns an empty table.
func NewTable() *Table {
	return &Table{
		byName: make(map[string]*Symbol),
		Defs:   make(map[ast.Node]*Symbol),
		Uses:   make(map[ast.Node]*Symbol),
	}
}

// Lookup finds a symbol by name.
func (t *Table) Lookup(name string) (*Symbol, bool) {
	s, ok := t.byName[name]
	return s, ok
}

// Symbols returns every symbol in definition order, with the names that were
// never defined last, in the order they were first used.
func (t *Table) Symbols() []*Symbol { return t.order }

// insert adds a symbol, or returns the one already there.
func (t *Table) insert(s *Symbol) (*Symbol, bool) {
	if prev, ok := t.byName[s.Name]; ok {
		return prev, false
	}
	t.byName[s.Name] = s
	t.order = append(t.order, s)
	return s, true
}

// define records a declaration. It returns the existing symbol and false when
// the name is already defined, so the caller can cite the first declaration -
// which is what a 2,500 line file needs from a duplicate-name diagnostic.
func (t *Table) define(node ast.Node, name string, kind Kind, pos token.Position) (*Symbol, bool) {
	sym := &Symbol{Name: name, Kind: kind, Def: pos, Type: ast.TypeOfName(name)}
	got, fresh := t.insert(sym)
	if !fresh {
		if got.Kind != Undefined {
			return got, false
		}
		// The name was used before it was declared, which L allows
		// everywhere. Fill in the symbol that the forward reference created
		// rather than making a second one, so its uses are not orphaned.
		got.Kind, got.Def, got.Type = kind, pos, sym.Type
	}
	if node != nil {
		t.Defs[node] = got
	}
	return got, true
}

// use records a reference, creating an Undefined symbol when the name has not
// been seen. That is the forward-reference queue of
// pkg/lowl/assembler/symtab.go in a different shape: nothing is resolved at
// the point of use, and the report comes after the whole program is walked.
func (t *Table) use(node ast.Node, name string, u Use, pos token.Position) *Symbol {
	sym, ok := t.byName[name]
	if !ok {
		sym = &Symbol{Name: name, Kind: Undefined, Type: ast.TypeOfName(name)}
		t.insert(sym)
	}
	sym.Refs = append(sym.Refs, Reference{Pos: pos, Use: u})
	if node != nil {
		t.Uses[node] = sym
	}
	return sym
}
