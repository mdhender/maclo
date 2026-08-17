// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package sema

import "github.com/mdhender/maclo/pkg/l/ast"

// The names L defines for itself.
//
// Without these, resolving the L source of ML/I reports fifteen names as
// undefined that are not: they are the language's own constants and the
// machine-dependent logic, which is specified in prose rather than written in
// L. Each entry cites where the manual says so.

// predefinedConstant is a name that stands for a value.
type predefinedConstant struct {
	Name string
	Doc  string
}

var predefinedConstants = []predefinedConstant{
	// switch constants (lmap.txt 3.3.4.1)
	{"TRUE", "lmap.txt 3.3.4.1: one"},
	{"FALSE", "lmap.txt 3.3.4.1: zero"},

	// pointer constants (lmap.txt 3.3.4.2)
	{"ZEROPT", "lmap.txt 3.3.4.2: at or below any possible pointer"},
	{"NULLPT", "lmap.txt 3.3.4.2: no possible pointer"},

	// character constants (lmap.txt 3.3.4.3)
	{"STOPCODE", "lmap.txt 3.3.4.3: the end of the source text"},

	// the markers on an operation macro or insert (lmap.txt 3.3.4.4)
	{"OPMK", "lmap.txt 3.3.4.4: an ordinary operation macro"},
	{"LOCMK", "lmap.txt 3.3.4.4: a local NEC macro"},
	{"UINSMK", "lmap.txt 3.3.4.4: an unprotected insert"},
	{"PINSMK", "lmap.txt 3.3.4.4: a protected insert"},
	{"STRMK", "lmap.txt 3.3.4.4: a straight-scan macro"},

	// the markers that may follow a delimiter (lmap.txt 3.3.4.4)
	{"ENDCHN", "lmap.txt 3.3.4.4: a closing delimiter, and the end of a chain"},
	{"EXCLMK", "lmap.txt 3.3.4.4: an exclusive delimiter"},
	{"WITHMK", "lmap.txt 3.3.4.4: the WITH keyword"},
	{"WTHSMK", "lmap.txt 3.3.4.4: the WITHS keyword"},
	{"SPCSMK", "lmap.txt 3.3.4.4: the SPACES keyword"},

	// the two that depend on the listing width (lmap.txt 3.3.4.4)
	{"TEXMAX", "lmap.txt 3.3.4.4: twice the listing width N"},
	{"HTMAX", "lmap.txt 3.3.4.4: the listing width N less four"},
}

// lengthMacros are the six constants that say how much storage each data type
// occupies (lmap.txt 3.3.1). They are legal only inside the argument of OF,
// which check.go enforces.
var lengthMacros = map[string]string{
	"LPT":  "lmap.txt 3.3.1: storage units in a pointer",
	"LNM":  "lmap.txt 3.3.1: storage units in a number",
	"LSW":  "lmap.txt 3.3.1: storage units in a switch",
	"LCH":  "lmap.txt 3.3.1: storage units in a character",
	"LICH": "lmap.txt 3.3.1: one over LCH",
	"LHV":  "lmap.txt 3.3.1: storage units in the hash table",
}

// mdLabels are the places in the machine-dependent logic that L branches to
// (lmap.txt 7.3).
var mdLabels = map[string]string{
	"MDHALT": "lmap.txt 7.3.3: the end of a process",
	"MDABRT": "lmap.txt 7.3.4: an abandoned process",
	"MDGOBC": "lmap.txt 7.1.5: go back a character",
}

// mdSub describes a machine-dependent subroutine well enough for the CALL
// agreement check: whether it takes an argument, and whether it has an exit.
type mdSub struct {
	Param   *ast.ParamSpec
	HasExit bool
	Doc     string
}

// mdSubs is the machine-dependent logic of chapter 7. The shapes were taken
// from the manual and cross-checked against every call in the L source of
// ML/I.
var mdSubs = map[string]mdSub{
	"MDTEST": {Param: &ast.ParamSpec{Name: "PARPT", Type: ast.PT}, HasExit: true, Doc: "lmap.txt 7.1.1"},
	"MDFIND": {Doc: "lmap.txt 7.1.2"},
	"MDCONV": {Doc: "lmap.txt 7.1.3"},
	"MDNUM":  {HasExit: true, Doc: "lmap.txt 7.1.4"},
	"MDERPR": {Doc: "lmap.txt 7.2.1"},
	"MDQUOT": {Doc: "lmap.txt 7.2.2"},
	"MDINIT": {Doc: "lmap.txt 7.3.2"},
	"MDOP":   {Doc: "lmap.txt 7.4"},
}

// heTableLabels are the three data labels HETABLES lays down. They are
// referenced through AD and defined nowhere in the source, because the
// statement generates them (lmap.txt 6.2.3.1).
//
// They are seeded only when the program actually contains a HETABLES, so that
// a program without one still has its uses of them reported. That turns a
// silent allowance into a checked one.
var heTableLabels = []string{"ERBLOC", "GHSHTB", "SVEC"}

// entryLabels are the labels the machine-dependent logic branches to in order
// to start the MI-logic: BEGIN, or MBEGIN when the INVALS SECTION has been
// deleted from the map (lmap.txt 7.3.1).
//
// Nothing in L branches to them, so without this they would be reported as
// labels nobody uses - which is true and useless.
var entryLabels = map[string]string{
	"BEGIN":  "lmap.txt 7.3.1: entered from the initialisation code",
	"MBEGIN": "lmap.txt 7.3.1: entered instead of BEGIN when INVALS is deleted",
}

// chainVars are the two variables a CHAIN FROM reads and writes without
// naming them (lmap.txt 4.2.2). They are declared in VARS like any other
// variable; what the seeding does is register the uses.
var chainVars = []string{"CHANPT", "CHLINK"}

// sectionClasses maps the ten SECTION names to what may appear in them
// (lmap.txt 2.4).
var sectionClasses = map[string]string{
	"VARS":     "vars",
	"INVALS":   "program",
	"MAIN":     "program",
	"MAINSUBS": "program",
	"OPMACS":   "program",
	"DEFSUBS":  "program",
	"ERR":      "program",
	"ENVPR":    "program",
	"MACNAMES": "data",
	"DELS":     "data",
}

// blockSizeName is the constant that gives the size of a declared block.
//
// The manual lists four of them by name (lmap.txt 3.3.4.4) and the L source of
// ML/I declares five blocks, so the fifth would be missing. Deriving the name
// from the declaration is both less code and more correct.
func blockSizeName(block string) string { return block + "SZ" }
