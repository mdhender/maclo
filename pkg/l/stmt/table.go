// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package stmt

// Entry describes one statement of L.
type Entry struct {
	// Kind is the statement. It must equal the entry's index in table.
	Kind Kind
	// Words is the spelling, split on spaces. Every multi-word head in L is
	// exactly two words, which is what makes Lookup a two-step longest match.
	Words []string
	// Cat is the manual's chapter grouping.
	Cat Category
	// Where is the set of SECTION classes the statement may appear in.
	Where Sections
	// Role says whether the statement opens or closes a nested construct.
	Role Role
	// Doc cites the manual.
	Doc string
}

// table is the one list. Its index is the Kind, which TestTableIndexMatchesKind
// checks, so the enum in kind.go and this cannot drift apart.
var table = [numKinds]Entry{
	Unknown: {Unknown, nil, CatNone, 0, RolePlain, ""},

	PrgStart: {PrgStart, []string{"PRGSTART"}, CatLayout, InFrame, RolePlain, "lmap.txt 2.4"},
	PrgEnd:   {PrgEnd, []string{"PRGEND"}, CatLayout, InFrame, RolePlain, "lmap.txt 2.4"},
	Section:  {Section, []string{"SECTION"}, CatLayout, InFrame, RoleOpen, "lmap.txt 2.4"},
	EndSect:  {EndSect, []string{"ENDSECT"}, CatLayout, InFrame, RoleClose, "lmap.txt 2.4"},

	Dec:      {Dec, []string{"DEC"}, CatDeclarative, InVARS, RolePlain, "lmap.txt 5.2.1"},
	Equate:   {Equate, []string{"EQUATE"}, CatDeclarative, InVARS, RolePlain, "lmap.txt 5.2.2"},
	BlockDec: {BlockDec, []string{"BLOCKDEC"}, CatDeclarative, InVARS, RoleOpen, "lmap.txt 5"},
	EndBlock: {EndBlock, []string{"ENDBLOCK"}, CatDeclarative, InVARS, RoleClose, "lmap.txt 5"},

	Subroutine:  {Subroutine, []string{"SUBROUTINE"}, CatRoutine, InProgram, RoleOpen, "lmap.txt 4.1.1.1"},
	LinkRoutine: {LinkRoutine, []string{"LINKROUTINE"}, CatRoutine, InProgram, RoleOpen, "lmap.txt 4.1.2.1"},
	EndSub:      {EndSub, []string{"ENDSUB"}, CatRoutine, InProgram, RoleClose, "lmap.txt 4.1.1.1"},
	ReturnFrom:  {ReturnFrom, []string{"RETURN", "FROM"}, CatRoutine, InProgram, RolePlain, "lmap.txt 4.1.1.2"},
	ExitFrom:    {ExitFrom, []string{"EXIT", "FROM"}, CatRoutine, InProgram, RolePlain, "lmap.txt 4.1.1.3"},
	LinkBack:    {LinkBack, []string{"LINK", "BACK"}, CatRoutine, InProgram, RolePlain, "lmap.txt 4.1.2.2"},
	Call:        {Call, []string{"CALL"}, CatRoutine, InProgram, RolePlain, "lmap.txt 4.1.3"},

	If:        {If, []string{"IF"}, CatCompound, InProgram, RoleOpen, "lmap.txt 4.2.1"},
	End:       {End, []string{"END"}, CatCompound, InProgram, RoleClose, "lmap.txt 4.2.1"},
	ChainFrom: {ChainFrom, []string{"CHAIN", "FROM"}, CatCompound, InProgram, RoleOpen, "lmap.txt 4.2.2"},
	EndCh:     {EndCh, []string{"ENDCH"}, CatCompound, InProgram, RoleClose, "lmap.txt 4.2.2"},

	MoveFrom:     {MoveFrom, []string{"MOVE", "FROM"}, CatMove, InProgram, RolePlain, "lmap.txt 4.3.1"},
	MStackFrom:   {MStackFrom, []string{"MSTACK", "FROM"}, CatMove, InProgram, RolePlain, "lmap.txt 4.3.2"},
	MUnstackFrom: {MUnstackFrom, []string{"MUNSTACK", "FROM"}, CatMove, InProgram, RolePlain, "lmap.txt 4.3.3"},

	Read:     {Read, []string{"READ"}, CatIO, InProgram, RolePlain, "lmap.txt 4.4.1"},
	OutputID: {OutputID, []string{"OUTPUTID"}, CatIO, InProgram, RolePlain, "lmap.txt 4.4.2"},
	PRText:   {PRText, []string{"PRTEXT"}, CatIO, InProgram, RolePlain, "lmap.txt 4.4.3"},

	Backspace: {Backspace, []string{"BACKSPACE"}, CatAssign, InProgram, RolePlain, "lmap.txt 4.5.1"},
	CharMatch: {CharMatch, []string{"CHARMATCH"}, CatAssign, InProgram, RolePlain, "lmap.txt 4.5.2"},
	GoTo:      {GoTo, []string{"GO", "TO"}, CatAssign, InProgram, RolePlain, "lmap.txt 4.5.3"},
	Scale:     {Scale, []string{"SCALE"}, CatAssign, InProgram, RolePlain, "lmap.txt 4.5.4"},
	Set:       {Set, []string{"SET"}, CatAssign, InProgram, RolePlain, "lmap.txt 4.5.5"},
	SetSW:     {SetSW, []string{"SETSW"}, CatAssign, InProgram, RolePlain, "lmap.txt 4.5.6"},
	Stack:     {Stack, []string{"STACK"}, CatAssign, InProgram, RolePlain, "lmap.txt 4.5.7"},
	Test:      {Test, []string{"TEST"}, CatAssign, InProgram, RolePlain, "lmap.txt 4.5.8"},
	Unstack:   {Unstack, []string{"UNSTACK"}, CatAssign, InProgram, RolePlain, "lmap.txt 4.5.9"},

	DC:       {DC, []string{"DC"}, CatData, InData, RolePlain, "lmap.txt 6.2.1"},
	LayChain: {LayChain, []string{"LAYCHAIN"}, CatData, InData, RolePlain, "lmap.txt 6.2.2"},
	HETables: {HETables, []string{"HETABLES"}, CatData, InData, RolePlain, "lmap.txt 6.2.3.1"},
	OpMac:    {OpMac, []string{"OPMAC"}, CatData, InData, RolePlain, "lmap.txt 6.2.4"},
}

// openedBy maps a closing statement to the openers it may close.
//
// ENDSUB is the reason this is a slice rather than a single Kind: LINKROUTINE
// has no closer of its own and ends with ENDSUB like a SUBROUTINE does
// (lmap.txt 4.1.2.1). That one fact is why the L source of ML/I holds 59
// ENDSUB for 58 SUBROUTINE.
var openedBy = map[Kind][]Kind{
	EndSect:  {Section},
	EndBlock: {BlockDec},
	EndSub:   {Subroutine, LinkRoutine},
	End:      {If},
	EndCh:    {ChainFrom},
}
