// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

// Package stmt is the statement vocabulary of L. It is the analogue of
// pkg/lowl/op, and it is deliberately built differently.
//
// pkg/lowl/op keeps three hand-written lists in step by hand: the enum in
// codes.go, the switch in stringer.go, and the map in lookup.go. CLAUDE.md
// documents the six-file ritual that adding an opcode therefore costs. Here
// there is one table, indexed by the enum, and a test that says so. String and
// Lookup both read it, so there is no second list to forget.
//
// The enum holds statement heads only. THEN, INIT, LENG, GOING, EXIT, the type
// suffixes and the macros OF, AD, IND, BLOCK, RL and LID are argument syntax
// and are absent on purpose: it means Lookup is called at exactly one position
// in the grammar, so EXIT as a keyword of SUBROUTINE can never be confused
// with EXIT FROM as a statement.
package stmt

// Kind identifies a statement of L. The zero value is Unknown.
type Kind uint8

// enums for Kind. The order here is the order of table in table.go, and
// TestTableIndexMatchesKind is what keeps the two in step.
const (
	Unknown Kind = iota

	// layout (lmap.txt 2.4)
	PrgStart
	PrgEnd
	Section
	EndSect

	// declarative (lmap.txt 5)
	Dec
	Equate
	BlockDec
	EndBlock

	// routines (lmap.txt 4.1)
	Subroutine
	LinkRoutine
	EndSub
	ReturnFrom
	ExitFrom
	LinkBack
	Call

	// compound (lmap.txt 4.2)
	If
	End
	ChainFrom
	EndCh

	// block moves (lmap.txt 4.3)
	MoveFrom
	MStackFrom
	MUnstackFrom

	// input and output (lmap.txt 4.4)
	Read
	OutputID
	PRText

	// assignment and branching (lmap.txt 4.5)
	Backspace
	CharMatch
	GoTo
	Scale
	Set
	SetSW
	Stack
	Test
	Unstack

	// the data SECTIONs (lmap.txt 6)
	DC
	LayChain
	HETables
	OpMac

	// numKinds is one past the last statement. It is not a statement.
	numKinds
)

// Category groups the statements the way the manual's chapters do. It is what
// a listing or a report uses to say what kind of thing went wrong, and it is
// not used for checking - Sections is.
type Category uint8

// enums for Category
const (
	CatNone Category = iota
	CatLayout
	CatDeclarative
	CatRoutine
	CatCompound
	CatMove
	CatIO
	CatAssign
	CatData
)

// String implements the Stringer interface.
func (c Category) String() string {
	switch c {
	case CatNone:
		return "none"
	case CatLayout:
		return "layout"
	case CatDeclarative:
		return "declarative"
	case CatRoutine:
		return "routine"
	case CatCompound:
		return "compound"
	case CatMove:
		return "move"
	case CatIO:
		return "io"
	case CatAssign:
		return "assign"
	case CatData:
		return "data"
	}
	return "category"
}

// Sections is the set of places a statement may appear. lmap.txt 2.4 divides
// the logic into ten SECTIONs of three classes: VARS holds declarations, the
// two data SECTIONs hold data-defining statements, and the rest hold
// executable statements. Carrying the rule in the table means sema gets the
// "DEC only in VARS" check without a switch of its own.
type Sections uint8

// enums for Sections
const (
	// InFrame is outside any SECTION: the four layout statements.
	InFrame Sections = 1 << iota
	// InVARS is the VARS SECTION.
	InVARS
	// InProgram is INVALS, MAIN, MAINSUBS, OPMACS, DEFSUBS, ERR and ENVPR.
	InProgram
	// InData is MACNAMES and DELS.
	InData
)

// Role says whether a statement opens a nested construct, closes one, or
// neither. ast/build.go pairs them; the table only records which is which.
type Role uint8

// enums for Role
const (
	RolePlain Role = iota
	RoleOpen
	RoleClose
)
