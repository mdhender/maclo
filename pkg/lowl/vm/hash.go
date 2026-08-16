// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm

// The hash table that ML/I looks its built-in macro names up in is built by
// the assembler, from the HASH and THASH statements, and searched at run time
// by MDFIND. The two must agree on which chain a name belongs to or a name
// that is present will not be found, so the bucket calculation lives here,
// once, and both callers use it.
//
// Supplement 3 leaves the algorithm to the implementer and suggests summing
// the length with the first and last characters. Nothing outside this file
// depends on the particular answer: ML/I only requires that a name hash to
// the same chain when it is stored and when it is looked up.

// LHV is the size of the hash table, measured the way the ML/I source measures
// it, so that OF(LHV) resolves. Supplement 3 gives 32*LNM as the usual choice
// and notes 16, 64 and 128 are also workable. It must be a power of two times
// LNM for the masking in HashName to stay in range.
const LHV = 32 * 1 // 32 * LNM, and LNM is 1 here

// HashName returns the offset, from the start of the hash table, of the chain
// head that name belongs on. The result is always a multiple of lnm and always
// less than LHV, which is what MDFIND promises the rest of ML/I.
//
// lch and lnm are passed in rather than assumed so that this keeps working if
// a character or a number ever stops being one word.
func HashName(name string, lch, lnm int) int {
	if len(name) == 0 {
		return 0
	}
	return hashAtom(len(name)*lch, int(name[0]), int(name[len(name)-1]), lnm)
}

// hashAtom is the calculation itself, in the terms MDFIND has it in: the
// length of the atom in characters times LCH, which is what IDLEN holds, and
// the first and last characters, which is what IDPT and SPT point at. It is
// the algorithm Supplement 3 suggests, and taking it apart this way is what
// lets the assembler and MDFIND run the same arithmetic over the same atom
// without one of them having to build a string.
func hashAtom(idlen, first, last, lnm int) int {
	n := ((idlen + first + last) * lnm) & (LHV - 1)
	// round down to a whole number of words; a chain head is a number, and a
	// pointer into the middle of one would be meaningless.
	return n - n%lnm
}
