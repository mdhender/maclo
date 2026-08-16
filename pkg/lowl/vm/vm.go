// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm

import (
	"github.com/mdhender/maclo/pkg/lowl/op"
	"io"
)

const (
	MAX_WORDS = 65_536
)

type VM struct {
	Name      string // name of the virtual machine
	PC        int
	A, B, C   int
	Registers struct {
		Cmp         CMPRSLT
		DSTPT       int // points to the variable holding the destination field pointer (stack moves)
		FFPT        int // points to the variable holding the first free location of the forwards stack
		LCH         int // length of a character in this machine
		LFPT        int // points to the variable holding the last location in use on the backwards stack
		LNM         int // length of a number in this machine
		NUMPT       int // first word of the storage MDCONV builds its decimal text in
		PARNM       int // points to the variable holding the subroutine parameter
		SRCPT       int // points to the variable holding the source field pointer (stack moves)
		SVARPT      int // points to the variable holding the address of the S-variable block; zero until SetSystemVariables lays one out
		Halted      bool
		JumpValue   int // jump value for GOTBL
		Start, Last int // starting, last address
		Cycles      int // how long Run may execute; DefaultCycles when zero
	}
	Streams struct {
		Stdout   io.Writer
		Messages io.Writer
		// Trace receives the machine's own commentary, which is not part of
		// either program stream. It is nil for a host that runs the machine
		// from buffers, because anything written here would corrupt them.
		Trace io.Writer
	}
	// Host supplies the machine dependent subroutines that reach outside the
	// machine: reading a character, writing one to the results stream, and
	// writing one to the messages stream. A nil Host falls back to the writers
	// Run was given, which is enough for a program such as LOWLTEST that only
	// writes messages.
	Host Host
	Core [MAX_WORDS]Word
	// Symbols maps every name the assembler resolved to an address onto that
	// address. The machine dependent subroutines are the only part of the
	// machine that needs it: they are written against the variables of the
	// program they serve (MEVAL, IDPT, HASHPT and the rest), and those live
	// wherever that program's DCL statements put them.
	Symbols map[string]int
	// RS is the return stack for subroutine calls. The forwards and backwards
	// stacks a LOWL program uses are not here: they live in the workspace
	// inside Core, reached through FFPT and LFPT.
	RS []int
}

type Word struct {
	Op       op.Code
	Value    int
	ValueTwo int // used by GOADD and GOBRPC
	Text     string
	Source   struct {
		Line         int
		Op           op.Code
		Parameters   string
		Continuation bool
	}
}
