// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

// Package lmap is the back end of the L front end: it maps L into LOWL.
//
// "L-map" is the manual's own word for this. L is machine independent and
// says nothing about how a statement is to be carried out; every statement in
// the manual carries an Action clause saying what the object code must do, and
// an implementor writes one mapping per object language. This package is the
// mapping whose object language is LOWL, which is the one pkg/lowl already
// assembles and runs.
//
// That choice is not arbitrary. ML/I is distributed twice: as L, which is what
// its logic is written in, and as LOWL, which is what an L-map like this one
// produced from it in 1971. So the output of this package can be read beside
// the output the authors published, and pkg/l/lmap/ml1aie_test.go does exactly
// that.
//
// The two stages mirror ast.Build and ast.WriteListing:
//
//	Map(prog, tab)        an *ast.Program -> a *Program of LOWL statements
//	(*Program).WriteLOWL  a *Program -> the text an assembler reads
//
// They are separate because a statement cannot be rendered until the whole
// program has been laid out. RL, the relative-location item the delimiter
// tables are built out of, carries the distance from itself to its target, and
// the assembler checks the distance the source claims against the one it lays
// out. So Map records the claim symbolically and fills it in once every word's
// position is known.
//
// # What the L-map has to supply
//
// L does not describe a whole program. Chapter 7 of the manual calls the rest
// the MD-logic: the character-level input and output, the chain walking that
// CHAIN FROM compiles into, the number and text conversions, the layout
// character table, and the code that runs before the logic does. None of it is
// in the L source and all of it is in prelude.go, written from the manual.
//
// # Nothing here writes a file
//
// WriteLOWL takes an io.Writer, as every listing under pkg/l does. The engine
// is handed to the assembler in a bytes.Buffer, and the only callers of
// os.Create are cmd/macl and cmd/lcheck, on paths a user named.
package lmap
