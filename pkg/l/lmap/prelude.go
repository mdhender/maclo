// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package lmap

import (
	"github.com/mdhender/maclo/pkg/lowl/op"
)

// The MD-logic.
//
// L does not describe a whole program. Chapter 7 of the manual lists what is
// left over -- the pieces of code that depend on the machine rather than on
// what ML/I does -- and says the implementor writes them. This file is that,
// for the machine pkg/lowl provides.
//
// Four of the manual's subroutines are not here, because this machine already
// has them: MDCONV, MDFIND and MDOP are instructions, and MDTEST is what GOPC
// branches on. Two more, MDREAD and MDOUCH, are instructions that reach the
// host, and what is here around them is the part the manual describes and the
// host does not do -- line counting, the startline character, and the stop
// code at the end of the input.
//
// The rest is either a statement of L that compiles into a call rather than
// into code -- CHAIN FROM is the whole of LOSCHN and LOECHN -- or one of the
// manual's own: MDERPR, MDNUM, MDGOBC and MDHALT.
//
// Written from the manual. The published LOWL of ML/I contains the same
// routines, because it was produced by an L-map like this one, and it is what
// pkg/l/lmap/ml1aie_test.go checks the answer against; it is not where the
// answer came from.

// The sizes the initialisation code works in.
const (
	// permanentVariables is how many of ML/I's permanent variables exist
	// before a macro asks for more. P1 to P10 are the ones the user's manual
	// says are always there.
	permanentVariables = 10

	// systemVariableTwo is which S-variable holds the number of the source
	// line being read. The vector runs backwards from the pointer, one entry
	// per variable, so this is also how far back it is.
	systemVariableTwo = 2
)

// nameNewLine is the switch that says the character about to be read is the
// first of a line. The READ statement keeps it, because the line count has to
// be stepped before the character arrives rather than after the newline that
// ended the line before it.
const nameNewLine = "NLSW"

// initialise emits the code that runs before the logic does.
//
// The manual's list is in 7.3.1: process the control statements, set up the
// stacks, reserve the areas ML/I hands out itself, and branch to BEGIN. There
// are no control statements here -- pkg/ml1 reads the command line and sets
// the S-variables before the machine starts -- and the stacks are already
// there, because pkg/lowl/vm lays the workspace out between FFPT and LFPT
// before the first instruction. What is left is the reserving, the zeroising,
// and the two things this L-map has to work out for itself: the markers, whose
// values fall out of where the tables landed, and the limits on local
// definitions.
//
// It ends by falling into the initialisation the MI-logic wrote for itself,
// which is why there is no branch at the bottom.
func (m *mapper) initialise() {
	e := func(code op.Code, args ...Arg) { m.p.emit(0, code, args...) }

	m.p.blank()
	m.p.label("BEGIN")
	m.p.comment(0, "the MD-logic: what has to be true before the MI-logic starts")

	// Reserve the error block and the permanent variables out of the front of
	// the workspace. The error block holds a copy of a block of variables, so
	// it has to be as big as the biggest one; the permanent variables are
	// counted downwards from PVARPT, which holds how many there are.
	e(op.LAV, word(freeForwards), word("X"))
	e(op.STV, word(nameErrBl), word("P"))
	e(op.AAL, ofArg(itoa(m.errorBlockSize())+"*LNM"))
	e(op.STV, word(nameTemp), word("P"))
	e(op.AAL, ofArg(itoa(permanentVariables)+"*LNM"))
	e(op.STV, word("PVARPT"), word("P"))
	e(op.SAV, word(nameErrBl))
	e(op.GOSUB, word(bumpForwards), num(0))
	e(op.LAL, num(permanentVariables))
	e(op.STV, word("PVNUM"), word("X"))

	// The end of the workspace, which is where a local definition stops being
	// valid, and the warning switch that says every construction is watched.
	e(op.LAV, word(freeBackwards), word("X"))
	e(op.STV, word("ENDPT"), word("X"))
	e(op.LAL, num(allWarningsOn))
	e(op.STV, word("GLBWSW"), word("X"))

	m.p.comment(0, "zeroise the permanent variables")
	m.p.label("LOZERO")
	e(op.BUMP, word(nameTemp), ofArg("LNM"))
	e(op.LAV, word(nameTemp), word("X"))
	e(op.CAV, word("PVARPT"), word("A"))
	e(op.GOEQ, word("LOMARK"), num(0), word("X"), word("X"))
	e(op.LAL, num(0))
	e(op.STI, word(nameTemp), word("X"))
	e(op.GO, word("LOZERO"), num(0), word("X"), word("X"))

	// The four markers L calls constants. They are values that cannot occur in
	// the input text, and the only one this L-map can name is the one the
	// assembler put in the tables for a composite name; the other three are
	// the numbers below it, which are equally impossible.
	m.p.comment(0, "the markers, counted down from the one the tables hold")
	m.p.label("LOMARK")
	e(op.LAA, word(nameCWith), word("C"))
	e(op.STV, word(nameTemp), word("X"))
	e(op.LAI, word(nameTemp), word("X"))
	for i, name := range markerOrder {
		if i > 0 {
			e(op.SAL, num(1))
		}
		flag := "P"
		if i == len(markerOrder)-1 {
			flag = "X"
		}
		e(op.STV, word(name), word(flag))
	}

	// The first character read starts a line.
	e(op.LAL, num(1))
	e(op.STV, word(nameNewLine), word("X"))

	// The S-variable vector is where the host put it, and AD of it means that
	// address rather than a table item.
	e(op.LAV, word("SVARPT"), word("X"))
	e(op.STV, word(nameSVec), word("X"))

	// Every construction is valid everywhere to begin with, which is what the
	// end of the workspace in each of the limit words says. The loop stops at
	// the first word that is not zero, which is the warning switch beyond them.
	m.p.comment(0, "no local definition is in scope yet")
	e(op.LAA, word("GHSHTB"), word("C"))
	e(op.AAL, ofArg("LHV"))
	e(op.STV, word(nameTemp), word("X"))
	m.p.label("LOLIMS")
	e(op.LAV, word("ENDPT"), word("X"))
	e(op.STI, word(nameTemp), word("X"))
	e(op.BUMP, word(nameTemp), ofArg("LNM"))
	e(op.LAI, word(nameTemp), word("X"))
	e(op.CAL, num(0))
	e(op.GOEQ, word("LOLIMS"), num(0), word("X"), word("X"))
	m.p.blank()
}

// errorBlockSize is how much storage the error block needs.
//
// It holds a copy of the block of variables an error message is printed out
// of, so the largest block is the size that is certainly enough. Reserving by
// the largest rather than by the one that is actually copied costs a few words
// of workspace and needs no rule about which block that is.
func (m *mapper) errorBlockSize() int {
	n := 1
	for _, size := range m.blockSize {
		if size > n {
			n = size
		}
	}
	return n
}

// prelude emits the subroutines and labels the MD-logic consists of.
func (m *mapper) prelude() {
	m.p.blank()
	m.p.comment(0, "the MD-logic: the code L leaves to the L-map")
	m.p.blank()
	m.chainRuntime()
	m.printText()
	m.readNumber()
	m.readCharacter()
	m.writeIdentifier()
	m.goByClass()
	m.finalise()
}

// chainRuntime emits the two halves of CHAIN FROM.
//
// A chain is a run of items each of which holds the distance to the next in
// its first word, and zero at the end. Starting one is loading that word;
// carrying on is adding it. Both say through their exits whether there was
// anything, which is what lets the statement they belong to be three
// instructions.
func (m *mapper) chainRuntime() {
	e := func(code op.Code, args ...Arg) { m.p.emit(0, code, args...) }

	m.p.comment(0, "CHAIN FROM: start a chain, the address of its head in A")
	e(op.SUBR, word(nameStartChain), word("PARNM"), num(2))
	e(op.STV, word("CHANPT"), word("P"))
	e(op.CAL, num(0))
	e(op.GOEQ, word("LOSCH1"), num(0), word("X"), word("X"))
	e(op.LAI, word("CHANPT"), word("X"))
	e(op.STV, word("CHLINK"), word("X"))
	e(op.EXIT, num(2), word(nameStartChain))
	m.p.label("LOSCH1")
	e(op.EXIT, num(1), word(nameStartChain))
	m.p.blank()

	m.p.comment(0, "ENDCH: step to the next item, if there is one")
	e(op.SUBR, word(nameEndChain), word("X"), num(2))
	e(op.LAV, word("CHLINK"), word("X"))
	e(op.CAL, num(0))
	e(op.GOEQ, word("LOECH1"), num(0), word("X"), word("X"))
	e(op.LAV, word("CHLINK"), word("R"))
	e(op.AAV, word("CHANPT"))
	e(op.STV, word("CHANPT"), word("X"))
	e(op.LAI, word("CHANPT"), word("X"))
	e(op.STV, word("CHLINK"), word("X"))
	e(op.EXIT, num(1), word(nameEndChain))
	m.p.label("LOECH1")
	e(op.EXIT, num(2), word(nameEndChain))
	m.p.blank()
}

// printText emits MDERPR, the routine every error message is printed through.
//
// The manual gives it IDPT and IDLEN, allows it to clobber IDPT, and asks for
// one thing beyond writing the characters: the startline character is not a
// character anybody typed, so it is shown as the four characters (SL) rather
// than sent to the stream.
func (m *mapper) printText() {
	e := func(code op.Code, args ...Arg) { m.p.emit(0, code, args...) }

	m.p.comment(0, "MDERPR: IDLEN characters from IDPT, on the message stream")
	e(op.SUBR, word(namePrintText), word("X"), num(1))
	e(op.LAV, word("IDPT"), word("X"))
	e(op.AAV, word("IDLEN"))
	e(op.STV, word(nameTemp), word("X"))
	m.p.label("LOEPLP")
	e(op.LAV, word("IDPT"), word("X"))
	e(op.CAV, word(nameTemp), word("A"))
	e(op.GOGE, word("LOEPEX"), num(0), word("X"), word("X"))
	e(op.LCI, word("IDPT"), word("X"))
	e(op.CCN, word("SLREP"))
	e(op.GOEQ, word("LOEPSL"), num(0), word("X"), word("X"))
	e(op.LCI, word("IDPT"), word("R"))
	e(op.GOSUB, word("MDERCH"), word("X"))
	m.p.label("LOEPON")
	e(op.BUMP, word("IDPT"), ofArg("LCH"))
	e(op.GO, word("LOEPLP"), num(0), word("X"), word("X"))
	m.p.label("LOEPSL")
	e(op.MESS, quoted("(SL)"))
	e(op.GO, word("LOEPON"), num(0), word("X"), word("X"))
	m.p.label("LOEPEX")
	e(op.EXIT, num(1), word(namePrintText))
	m.p.blank()
}

// readNumber emits MDNUM, which reads the atom between IDPT and SPT as a
// decimal number.
//
// The manual splits the failures in two, and the split is what the exits are
// for. A first character that is not a digit means the atom was never meant to
// be a number, and the caller has somewhere else to try; a later one means it
// was and is wrong, which is an error in the input rather than a possibility
// the caller was exploring.
func (m *mapper) readNumber() {
	e := func(code op.Code, args ...Arg) { m.p.emit(0, code, args...) }

	m.p.comment(0, "MDNUM: the atom from IDPT to SPT, as a number, into MEVAL")
	e(op.SUBR, word(nameNumber), word("X"), num(2))
	e(op.LCI, word("IDPT"), word("X"))
	e(op.GOND, word("LONMNO"), num(0), word("X"), word("X"))
	e(op.STV, word("MEVAL"), word("X"))
	e(op.LAV, word("IDPT"), word("X"))
	e(op.STV, word(nameTemp), word("X"))
	m.p.label("LONMLP")
	e(op.BUMP, word(nameTemp), ofArg("LCH"))
	e(op.LAV, word("SPT"), word("X"))
	e(op.CAV, word(nameTemp), word("A"))
	e(op.GOLT, word("LONMOK"), num(0), word("X"), word("X"))
	e(op.LCI, word(nameTemp), word("X"))
	e(op.GOND, word(illegalArgument), num(0), word("E"), word("X"))
	e(op.STV, word(nameStore), word("X"))
	e(op.LAV, word("MEVAL"), word("X"))
	e(op.MULTL, num(10))
	e(op.AAV, word(nameStore))
	e(op.STV, word("MEVAL"), word("X"))
	e(op.GO, word("LONMLP"), num(0), word("X"), word("X"))
	m.p.label("LONMOK")
	e(op.EXIT, num(2), word(nameNumber))
	m.p.label("LONMNO")
	e(op.EXIT, num(1), word(nameNumber))
	m.p.blank()
}

// illegalArgument is where the MI-logic reports an argument it cannot use. The
// manual names it in MDNUM's action clause.
const illegalArgument = "ERLIA"

// readCharacter emits the READ statement.
//
// The manual gives this more to do than its name suggests. Reading the
// character is one instruction; around it are the things that make the input
// look like one long string to the logic -- a newline at the end of every
// line, the stop code at the end of the text -- and the line accounting, which
// has to happen before the first character of a line rather than after the
// newline that ended the one before, because that newline is itself a
// character of the line it ends.
func (m *mapper) readCharacter() {
	e := func(code op.Code, args ...Arg) { m.p.emit(0, code, args...) }

	m.p.comment(0, "the READ statement: one character, on to the forwards stack")
	e(op.SUBR, word(nameRead), word("X"), num(1))
	e(op.LAV, word(nameNewLine), word("X"))
	e(op.CAL, num(0))
	e(op.GOEQ, word("LORDGO"), num(0), word("X"), word("X"))

	m.p.comment(0, "a line is starting, so step the count of them")
	e(op.LAV, word("SVARPT"), word("X"))
	e(op.SAL, ofArg(itoa(systemVariableTwo)+"*LNM"))
	e(op.STV, word(nameTemp), word("X"))
	e(op.LAI, word(nameTemp), word("X"))
	e(op.AAL, num(1))
	e(op.STI, word(nameTemp), word("X"))
	e(op.LAV, word("LINECT"), word("X"))
	e(op.CAV, word("TLINCT"), word("X"))
	e(op.GONE, word("LORDLC"), num(0), word("X"), word("X"))
	e(op.LAI, word(nameTemp), word("X"))
	e(op.STV, word("TLINCT"), word("X"))
	m.p.label("LORDLC")
	e(op.LAI, word(nameTemp), word("X"))
	e(op.STV, word("LINECT"), word("X"))

	m.p.comment(0, "S1 asks for the imaginary character that marks a line start")
	e(op.LBV, word(nameTemp))
	e(op.LAM, ofArg("LNM"))
	e(op.CAL, num(0))
	e(op.GOEQ, word("LORDGO"), num(0), word("X"), word("X"))
	e(op.LCN, word("SLREP"))
	e(op.CFSTK)

	m.p.label("LORDGO")
	e(op.LAV, word(freeForwards), word("X"))
	e(op.STV, word(nameTemp), word("X"))
	e(op.GOSUB, word("MDREAD"), word("X"))
	e(op.GO, word("LORDND"), num(0), word("X"), word("C"))
	e(op.CFSTK)
	e(op.LAL, num(0))
	e(op.STV, word(nameNewLine), word("X"))
	e(op.LCI, word(nameTemp), word("X"))
	e(op.CCN, word("NLREP"))
	e(op.GONE, word("LORDEX"), num(0), word("X"), word("X"))
	e(op.LAL, num(1))
	e(op.STV, word(nameNewLine), word("X"))
	m.p.label("LORDEX")
	e(op.EXIT, num(1), word(nameRead))

	m.p.comment(0, "the end of the input: a newline if the last line wanted one, then the stop code")
	m.p.label("LORDND")
	e(op.LAV, word("SPT"), word("X"))
	e(op.STV, word(freeForwards), word("X"))
	e(op.LAV, word(nameNewLine), word("X"))
	e(op.CAL, num(0))
	e(op.GONE, word("LORDST"), num(0), word("X"), word("X"))
	e(op.LCN, word("NLREP"))
	e(op.CFSTK)
	m.p.label("LORDST")
	e(op.LCN, word("STOPCD"))
	e(op.CFSTK)
	e(op.EXIT, num(1), word(nameRead))
	m.p.blank()
}

// writeIdentifier emits the OUTPUTID statement.
//
// The manual asks for IDLEN characters from IDPT, says both may be clobbered,
// and says startline characters are ignored: they were never in the input, so
// they must not reach the output either.
func (m *mapper) writeIdentifier() {
	e := func(code op.Code, args ...Arg) { m.p.emit(0, code, args...) }

	m.p.comment(0, "the OUTPUTID statement: IDLEN characters from IDPT, on the results stream")
	e(op.SUBR, word(nameOutput), word("X"), num(1))
	e(op.LAV, word("IDPT"), word("X"))
	e(op.AAV, word("IDLEN"))
	e(op.STV, word(nameTemp), word("X"))
	m.p.label("LOOULP")
	e(op.LAV, word("IDPT"), word("X"))
	e(op.CAV, word(nameTemp), word("A"))
	e(op.GOGE, word("LOOUEX"), num(0), word("X"), word("X"))
	e(op.LCI, word("IDPT"), word("X"))
	e(op.CCN, word("SLREP"))
	e(op.GOEQ, word("LOOUON"), num(0), word("X"), word("X"))
	e(op.LCI, word("IDPT"), word("R"))
	e(op.GOSUB, word("MDOUCH"), word("X"))
	m.p.label("LOOUON")
	e(op.BUMP, word("IDPT"), ofArg("LCH"))
	e(op.GO, word("LOOULP"), num(0), word("X"), word("X"))
	m.p.label("LOOUEX")
	e(op.EXIT, num(1), word(nameOutput))
	m.p.blank()
}

// The labels MDGOBC answers through. They belong to the MI-logic, and the
// manual names all three in its description of the routine.
const (
	classSucceeded = "GOSUC"
	classFailed    = "GOFAIL"
)

// goByClass emits MDGOBC, which answers whether a piece of text belongs to one
// of the three classes the BC delimiter of the GO macro can ask about.
//
// The text runs from INFFPT to ERIAPT and the class is the single character at
// IDPT: L for a letter, N for a number, I for an identifier. The three differ
// only in what each character of the text is allowed to be, so they are one
// loop with a number saying what is wanted: an identifier takes any character
// an atom can be made of, a letter takes those that are not digits, and a
// number takes those that are, after a run of signs which the manual says to
// accept.
func (m *mapper) goByClass() {
	e := func(code op.Code, args ...Arg) { m.p.emit(0, code, args...) }

	m.p.comment(0, "MDGOBC: does the text from INFFPT to ERIAPT belong to the class at IDPT")
	m.p.label(nameGoByClass)
	e(op.LAV, word("INFFPT"), word("X"))
	e(op.STV, word(nameTemp), word("X"))
	e(op.LAL, num(0))
	e(op.STV, word(nameStore), word("X"))
	e(op.LCI, word("IDPT"), word("X"))
	e(op.CCL, quoted("I"))
	e(op.GOEQ, word("LOBCGO"), num(0), word("X"), word("X"))
	e(op.LCI, word("IDPT"), word("R"))
	e(op.CCL, quoted("L"))
	e(op.GOEQ, word("LOBCLT"), num(0), word("X"), word("X"))
	e(op.LCI, word("IDPT"), word("R"))
	e(op.CCL, quoted("N"))
	e(op.GONE, word(illegalArgument), num(0), word("X"), word("X"))

	m.p.comment(0, "a number may begin with any number of signs")
	m.p.label("LOBCSN")
	e(op.LCI, word(nameTemp), word("X"))
	e(op.CCL, quoted("+"))
	e(op.GOEQ, word("LOBCSK"), num(0), word("X"), word("X"))
	e(op.LCI, word(nameTemp), word("R"))
	e(op.CCL, quoted("-"))
	e(op.GONE, word("LOBCNM"), num(0), word("X"), word("X"))
	m.p.label("LOBCSK")
	e(op.BUMP, word(nameTemp), ofArg("LCH"))
	e(op.GO, word("LOBCSN"), num(0), word("X"), word("X"))

	m.p.label("LOBCNM")
	e(op.BUMP, word(nameStore), num(1))
	m.p.label("LOBCLT")
	e(op.BUMP, word(nameStore), num(1))

	m.p.comment(0, "a null string belongs to nothing")
	m.p.label("LOBCGO")
	e(op.LAV, word(nameTemp), word("X"))
	e(op.CAV, word("ERIAPT"), word("A"))
	e(op.GOEQ, word(classFailed), num(0), word("X"), word("X"))

	m.p.label("LOBCLP")
	e(op.LCI, word(nameTemp), word("X"))
	e(op.GOPC, word(classFailed), num(0), word("X"), word("X"))
	e(op.LAV, word(nameStore), word("X"))
	e(op.CAL, num(0))
	e(op.GOEQ, word("LOBCOK"), num(0), word("X"), word("X"))
	e(op.LAL, num(1))
	e(op.STV, word(nameLeng), word("X"))
	e(op.LCI, word(nameTemp), word("X"))
	e(op.GOND, word("LOBCTS"), num(0), word("X"), word("X"))
	e(op.BUMP, word(nameLeng), num(1))
	m.p.label("LOBCTS")
	e(op.LAV, word(nameLeng), word("X"))
	e(op.CAV, word(nameStore), word("X"))
	e(op.GONE, word(classFailed), num(0), word("X"), word("X"))

	m.p.label("LOBCOK")
	e(op.BUMP, word(nameTemp), ofArg("LCH"))
	e(op.LAV, word(nameTemp), word("X"))
	e(op.CAV, word("ERIAPT"), word("A"))
	e(op.GOEQ, word(classSucceeded), num(0), word("X"), word("X"))
	e(op.GO, word("LOBCLP"), num(0), word("X"), word("X"))
	m.p.blank()
}

// finalise emits MDHALT, which the manual also makes MDABRT.
//
// It is the end of every process, orderly or not. The manual asks for the
// count of calls and the count of lines, and lets the implementation print the
// list of constructions that were defined; both go on the message stream, and
// pkg/ml1 decides through S18 how much of it a reader asked for.
func (m *mapper) finalise() {
	e := func(code op.Code, args ...Arg) { m.p.emit(0, code, args...) }

	m.p.comment(0, "MDHALT, and MDABRT with it: the end of the process")
	m.p.label(nameHalt)
	e(op.MESS, quoted("$$AT END OF PROCESS: "))

	// The line being read is counted, and it is the one after the last one
	// there was, so the number to print is one less.
	e(op.LBV, word("SVARPT"))
	e(op.SBL, ofArg(itoa(systemVariableTwo)+"*LNM"))
	e(op.LAM, num(0))
	e(op.SAL, num(1))
	e(op.GOSUB, word(printNumber), num(0))
	e(op.MESS, quoted(" LINES, "))
	e(op.LAV, word("INVOCT"), word("X"))
	e(op.GOSUB, word(printNumber), num(0))
	e(op.MESS, quoted(" CALLS$"))
	e(op.GOSUB, word(printEnvironment), num(0))
	e(op.GOSUB, word("MDQUIT"), word("X"))
	m.p.blank()
}

// The two subroutines of the MI-logic the finalisation code calls. Printing a
// number is not something LOWL does, and the list of constructions is written
// by the SECTION that exists to write it.
const (
	printNumber      = "PRNUM"
	printEnvironment = "PRENV"
)
