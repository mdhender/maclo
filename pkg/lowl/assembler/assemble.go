// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

// Package assembler assembles the instructions and returns a VM that can run them.
package assembler

import (
	"bytes"
	"fmt"
	"github.com/mdhender/maclo/pkg/lowl/ast"
	"github.com/mdhender/maclo/pkg/lowl/op"
	"github.com/mdhender/maclo/pkg/lowl/vm"
	"io"
	"math"
	"os"
	"sort"
)

// Options controls the assembler's side effects.
//
// Both of them are off in the zero value, which is what a library caller
// wants: a caller that assembles in process, from buffers, must not print to
// anyone's terminal or leave listing files in whatever directory it happens to
// have been run from.
type Options struct {
	// Trace receives the assembler's commentary. Nil is silent.
	Trace io.Writer

	// Listings asks for asm_listing.txt and asm_symtab.txt to be written to
	// the current working directory.
	Listings bool
}

func Assemble(nodes ast.Nodes, opts Options) (*vm.VM, error) {
	tracef := func(format string, args ...any) {
		if opts.Trace != nil {
			_, _ = fmt.Fprintf(opts.Trace, format, args...)
		}
	}

	// create symbol table and initialize it with required constants
	symtab := newSymbolTable()
	symtab.InsertConstant(-1, "LCH", 1)       // LCH is the length (in words) of a character
	symtab.InsertConstant(-1, "LNM", 1)       // LMN is the length (in words) of a number
	symtab.InsertConstant(-1, "LICH", 1)      // LICH is the inverse of LCH
	symtab.InsertConstant(-1, "LHV", vm.LHV)  // LHV is the size of the hash table
	symtab.InsertConstant(-1, "NLREP", '\n')  // new-line
	symtab.InsertConstant(-1, "QUTREP", '"')  // quote mark
	symtab.InsertConstant(-1, "SPREP", ' ')   // space
	symtab.InsertConstant(-1, "TABREP", '\t') // tab
	// SLREP and STOPCD must be codes that cannot occur in the source text.
	// A character occupies a whole word here, so anything above the byte
	// range is safe: no input character can ever compare equal to one.
	symtab.InsertConstant(-1, "SLREP", 256)  // stands for a deleted character
	symtab.InsertConstant(-1, "STOPCD", 257) // stop code

	machine := vm.New()

	// the current subroutine name is set whenever we get a SUBR instruction.
	// it is used as a sanity check in the EXIT calls
	var currSubroutine struct {
		name          string
		numberOfExits int
	}
	jumpTable := 0

	// The hash table is built in two steps. Every HASH statement reserves one
	// word and records the name it stands for; THASH reserves the chain heads.
	// Neither can be filled in on the spot, because a chain link points at the
	// next entry on the chain and that entry has not been seen yet, so both
	// are resolved in one pass after the source has been read.
	type hashEntry struct {
		address int    // the word the HASH statement reserved
		name    string // the built-in macro name it stands for
		line    int
	}
	var hashEntries []hashEntry
	thashAddress := -1 // where THASH put the chain heads; -1 until it appears

	// assemble all the instructions
	for _, node := range nodes {
		// provide a default word for the instruction
		word := vm.Word{Op: node.Op} // default word to the current opcode
		// debugging
		word.Source.Line = node.Line
		word.Source.Op = node.Op
		word.Source.Parameters = node.Parameters.String()

		// emit the word
		switch node.Op {

		// this section implements instructions that look like "OP"
		case op.ALIGN:
			// ALIGN emits no code
		case op.BMOVE, op.FMOVE:
			if sym, ok := symtab.Lookup("SRCPT"); !ok {
				return nil, fmt.Errorf("%d: %d: internal error: SRCPT undefined", node.Line, node.Col)
			} else {
				word.Value = sym.address
			}
			if sym, ok := symtab.Lookup("DSTPT"); !ok {
				return nil, fmt.Errorf("%d: %d: internal error: DSTPT undefined", node.Line, node.Col)
			} else {
				word.ValueTwo = sym.address
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1
		case op.BSTK, op.CFSTK, op.FSTK:
			if _, ok := symtab.Lookup("FFPT"); !ok {
				return nil, fmt.Errorf("%d: %d: internal error: FFPT undefined", node.Line, node.Col)
			}
			if _, ok := symtab.Lookup("LFPT"); !ok {
				return nil, fmt.Errorf("%d: %d: internal error: LFPT undefined", node.Line, node.Col)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1
		case op.CSS:
			machine.Core[machine.PC], machine.PC = word, machine.PC+1
		case op.LINKB:
			// return from the linkroutine by branching to the address the
			// matching LINKR left in LINKPT.
			symtab.AddReference("LINKPT", machine.PC)
			machine.Core[machine.PC], machine.PC = word, machine.PC+1
		case op.THASH:
			if thashAddress != -1 {
				return nil, fmt.Errorf("%d: %s: table already defined on line %d", node.Line, node.Op, machine.Core[thashAddress].Source.Line)
			}
			// reserve one word per chain head. They stay zero unless a HASH
			// statement puts an entry on that chain.
			thashAddress = machine.PC
			for i := 0; i < vm.LHV; i += machine.Registers.LNM {
				machine.Core[machine.PC], machine.PC = word, machine.PC+1
				word.Source.Continuation = true
			}
		case op.WTHS:
			// a marker that cannot be mistaken for a length or a pointer.
			word.Value = math.MaxInt32
			machine.Core[machine.PC], machine.PC = word, machine.PC+1
		case op.PRGEN:
			machine.Core[machine.PC], machine.PC = vm.Word{Op: op.HALT}, machine.PC+1
		case op.GOTBL, op.MDCONV, op.MDERCH, op.MDFIND, op.MDOP, op.MDOUCH, op.MDQUIT, op.MDREAD, op.NOOP, op.UNKNOWN:
			// some op codes are not available to callers
			return nil, fmt.Errorf("%d: %d: %s: internal error", node.Line, node.Col, node.Op)

		// this section implements instructions that look like "OP (CONSTANT_VAR|NUMBER)"
		case op.ANDL, op.CCN, op.LCN, op.NCH:
			if minArgs := 1; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			// operand must be a constant
			switch constant := node.Parameters[0]; constant.Kind {
			case ast.Number:
				word.Value = constant.Number
			case ast.Variable:
				sym, ok := symtab.Lookup(constant.Text)
				if !ok {
					return nil, fmt.Errorf("%d: %s: %s %q: forward declaration not allowed here", node.Line, node.Op, constant.Kind, constant.Text)
				}
				switch sym.kind {
				case "constant":
					word.Value = sym.constant
				default:
					return nil, fmt.Errorf("%d: %s: %s: must be constant", node.Line, node.Op, constant.Text)
				}
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, constant.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		// this section implements instructions that look like "OP (CONSTANT_VAR|NUMBER|N-OF)"
		case op.AAL, op.CAL, op.CON, op.LAL, op.LAM, op.LCM, op.MULTL, op.ORL, op.SAL, op.SBL:
			if minArgs := 1; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch nOF := node.Parameters[0]; nOF.Kind {
			case ast.Macro:
				if minArgs := 2; len(node.Parameters) < minArgs {
					return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
				}
				expr := node.Parameters[1]
				value, err := evalMacro(nOF.Text, expr, symtab.GetEnv())
				if err != nil {
					return nil, fmt.Errorf("%d: %s: %s %s: %w", node.Line, node.Op, nOF.Kind, nOF.Text, err)
				}
				word.Value = value
			case ast.Number:
				word.Value = nOF.Number
			case ast.Variable:
				// variable must be a constant
				sym, ok := symtab.Lookup(nOF.Text)
				if !ok {
					return nil, fmt.Errorf("%d: %s: %s %q: forward declaration not allowed here", node.Line, node.Op, nOF.Kind, nOF.Text)
				}
				switch sym.kind {
				case "constant":
					word.Value = sym.constant
				default:
					return nil, fmt.Errorf("%d: %s: %s: must be constant", node.Line, node.Op, nOF.Text)
				}
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, nOF.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		// this section implements instructions that look like "OP LABEL"
		case op.DCL:
			if minArgs := 1; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch label := node.Parameters[0]; label.Kind {
			case ast.Variable:
				switch label.Text {
				case "DSTPT":
					machine.Registers.DSTPT = machine.PC
				case "FFPT":
					machine.Registers.FFPT = machine.PC
				case "LFPT":
					machine.Registers.LFPT = machine.PC
				case "PARNM":
					machine.Registers.PARNM = machine.PC
				case "SRCPT":
					machine.Registers.SRCPT = machine.PC
				default:
				}
				if ok := symtab.InsertAddress(node.Line, label.Text, machine.PC); !ok {
					return nil, fmt.Errorf("%d: %s: internal error: %s %q redefined", node.Line, node.Op, label.Kind, label.Text)
				}
				word.Text = label.Text
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, label.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1
		case op.MDLABEL:
			if minArgs := 1; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch name := node.Parameters[0]; name.Kind {
			case ast.Label:
				if _, ok := symtab.Lookup(name.Text); ok {
					return nil, fmt.Errorf("%d: %s: internal error: %s redefined", node.Line, node.Op, name.Kind)
				}
				symtab.InsertAddress(node.Line, name.Text, machine.PC)
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, name.Kind)
			}
			// MDLABEL emits no code

		// this section implements instructions that look like "OP LABEL N-OF"
		case op.RL:
			if minArgs := 3; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			// the distance the source says the label is at. It is checked
			// against the one the assembler lays out, not used in its place.
			var offset int
			switch nOF := node.Parameters[1]; nOF.Kind {
			case ast.Macro:
				value, err := evalMacro(nOF.Text, node.Parameters[2], symtab.GetEnv())
				if err != nil {
					return nil, fmt.Errorf("%d: %s: %s %s: %w", node.Line, node.Op, nOF.Kind, nOF.Text, err)
				}
				offset = value
			case ast.Number:
				offset = nOF.Number
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, nOF.Kind)
			}
			switch label := node.Parameters[0]; label.Kind {
			case ast.Variable:
				symtab.AddRelativeReference(label.Text, machine.PC, offset)
				word.Text = label.Text
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, label.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		// this section implements instructions that look like "OP LABEL"
		case op.LINKR:
			if minArgs := 1; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch name := node.Parameters[0]; name.Kind {
			case ast.Variable:
				if _, ok := symtab.Lookup(name.Text); !ok {
					if ok := symtab.InsertAddress(node.Line, name.Text, machine.PC); !ok {
						return nil, fmt.Errorf("%d: %s: internal error: %s redefined", node.Line, node.Op, name.Kind)
					}
				} else {
					symtab.UpdateAddress(name.Text, machine.PC)
				}
				word.Text = name.Text
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, name.Kind)
			}
			// a linkroutine returns through LINKPT rather than through the
			// subroutine stack, so entering one moves the return address there.
			symtab.AddReference("LINKPT", machine.PC)
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		// this section implements instructions that look like "OP LABEL FLAG(PARNM|X) NUMBER"
		case op.SUBR:
			if minArgs := 3; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch name := node.Parameters[0]; name.Kind {
			case ast.Variable:
				if _, ok := symtab.Lookup(name.Text); !ok {
					if ok := symtab.InsertAddress(node.Line, name.Text, machine.PC); !ok {
						return nil, fmt.Errorf("%d: %s: internal error: %s redefined", node.Line, node.Op, name.Kind)
					}
				} else {
					symtab.UpdateAddress(name.Text, machine.PC)
				}
				// add subroutine name for debugging
				word.Text = name.Text
				currSubroutine.name = name.Text
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, name.Kind)
			}
			switch flag := node.Parameters[1]; flag.Kind {
			case ast.Variable:
				switch flag.Text {
				case "X": // no parameters
					// emit a noop
					word.Op = op.NOOP
				case "PARNM": // named parameter
					// emit code to store register A into the named parameter
					symtab.AddReference(flag.Text, machine.PC)
					word.Op = op.STV
				default:
					return nil, fmt.Errorf("%d: %s: invalid parameter %q", node.Line, node.Op, flag.Text)
				}
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, flag.Kind)
			}
			switch exits := node.Parameters[2]; exits.Kind {
			case ast.Number:
				currSubroutine.numberOfExits = exits.Number
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, exits.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		// this section implements instructions that look like "OP LABEL FLAG(NUMBER|X)"
		case op.GOSUB:
			if minArgs := 2; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch label := node.Parameters[0]; label.Kind {
			case ast.Variable:
				switch label.Text {
				// the machine-dependent subroutines are never declared in the
				// source, so they are recognized here at the call site and
				// turned into a single instruction the host implements.
				case "MDCONV":
					word.Op = op.MDCONV
				case "MDERCH":
					word.Op = op.MDERCH
				case "MDFIND":
					word.Op = op.MDFIND
				case "MDOP":
					word.Op = op.MDOP
				case "MDOUCH":
					word.Op = op.MDOUCH
				case "MDQUIT":
					word.Op = op.MDQUIT
				case "MDREAD":
					word.Op = op.MDREAD
				default:
					symtab.AddReference(label.Text, machine.PC)
				}
			default:
				return nil, fmt.Errorf("%d: %s: %s: not allowed", node.Line, node.Op, label.Kind)
			}
			switch flag := node.Parameters[1]; flag.Kind {
			case ast.Number:
				// no action needed
			case ast.Variable:
				switch flag.Text {
				case "X":
					// MD logic should have been handled in label code above
				default:
					return nil, fmt.Errorf("%d: %s: flag: want X: got %q", node.Line, node.Op, flag.Text)
				}
			default:
				return nil, fmt.Errorf("%d: %s: flag: want X or NUMBER: got %q", node.Line, node.Op, flag.Kind)
			}
			jumpTable = 0 // reset the jump table
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		// this section implements instructions that look like "OP LABEL VARIABLE"
		case op.EQU:
			if minArgs := 2; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			var name string
			switch label := node.Parameters[0]; label.Kind {
			case ast.Variable:
				if _, ok := symtab.Lookup(label.Text); ok {
					return nil, fmt.Errorf("%d: %s: internal error: %q redefined", node.Line, node.Op, label.Text)
				}
				name = label.Text
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, label.Kind)
			}
			switch v := node.Parameters[1]; v.Kind {
			case ast.Variable:
				symtab.InsertAlias(node.Line, name, v.Text)
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, v.Kind)
			}
			// EQU emits no code

		// this section implements instructions that look like "OP NUMBER LABEL"
		case op.EXIT:
			if minArgs := 2; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch exits := node.Parameters[0]; exits.Kind {
			case ast.Number:
				if exits.Number < 1 {
					return nil, fmt.Errorf("%d: %s: exit-number %d: invalid", node.Line, node.Op, exits.Number)
				} else if exits.Number > currSubroutine.numberOfExits {
					return nil, fmt.Errorf("%d: %s: exit-number %d: exceeds %d", node.Line, node.Op, exits.Number, currSubroutine.numberOfExits)
				}
				word.Value = exits.Number
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, exits.Kind)
			}
			// machine expects that label will match the subroutine name that we're currently in
			switch label := node.Parameters[1]; label.Kind {
			case ast.Variable:
				if label.Text != currSubroutine.name {
					return nil, fmt.Errorf("%d: %s: exit wants %q: got %q", node.Line, node.Op, currSubroutine.name, label.Text)
				}
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, label.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		// this section implements instructions that look like "OP QUOTED_TEXT"
		case op.CCL:
			if minArgs := 1; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch text := node.Parameters[0]; text.Kind {
			case ast.QuotedText:
				if len(text.Text) != 1 {
					return nil, fmt.Errorf("%d: %s: want single character: got %q", node.Line, node.Op, text.Text)
				}
				word.Value = int(text.Text[0])
			default:
				return nil, fmt.Errorf("%d: %s: %s: not allowed", node.Line, node.Op, text.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1
		case op.MESS:
			if minArgs := 1; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch text := node.Parameters[0]; text.Kind {
			case ast.QuotedText:
				word.Text = text.Text
			default:
				return nil, fmt.Errorf("%d: %s: %s: not allowed", node.Line, node.Op, text.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1
		case op.HASH:
			if minArgs := 1; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch text := node.Parameters[0]; text.Kind {
			case ast.QuotedText:
				// the value is the link to the next entry on this name's
				// chain, and is filled in once every entry has been seen.
				word.Text = text.Text
				hashEntries = append(hashEntries, hashEntry{address: machine.PC, name: text.Text, line: node.Line})
			default:
				return nil, fmt.Errorf("%d: %s: %s: not allowed", node.Line, node.Op, text.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1
		case op.NB: // ignore comments
			if minArgs := 1; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch text := node.Parameters[0]; text.Kind {
			case ast.QuotedText:
				// comments are ignored
			default:
				return nil, fmt.Errorf("%d: %s: %s: not allowed", node.Line, node.Op, text.Kind)
			}
			// NB emits no code
		case op.PRGST:
			if minArgs := 1; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch text := node.Parameters[0]; text.Kind {
			case ast.QuotedText:
				// no action needed
			default:
				return nil, fmt.Errorf("%d: %s: %s: not allowed", node.Line, node.Op, text.Kind)
			}
			// PRGST emits no code
		case op.STR:
			if minArgs := 1; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch text := node.Parameters[0]; text.Kind {
			case ast.QuotedText:
				for _, ch := range text.Text {
					word.Value = int(ch)
					machine.Core[machine.PC], machine.PC = word, machine.PC+1
					word.Source.Continuation = true
				}
			default:
				return nil, fmt.Errorf("%d: %s: %s: not allowed", node.Line, node.Op, text.Kind)
			}
			// STR emits code in the text loop above

		// this section implements instructions that look like "OP VARIABLE"
		case op.AAV, op.ABV, op.ANDV, op.CCI, op.CLEAR, op.LBV, op.SAV, op.SBV, op.UNSTK:
			if minArgs := 1; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch v := node.Parameters[0]; v.Kind {
			case ast.Variable:
				symtab.AddReference(v.Text, machine.PC)
			default:
				return nil, fmt.Errorf("%d: %s: %s: not allowed", node.Line, node.Op, v.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1
		case op.GOADD:
			if minArgs := 1; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch v := node.Parameters[0]; v.Kind {
			case ast.Variable:
				symtab.AddReference(v.Text, machine.PC)
			default:
				return nil, fmt.Errorf("%d: %s: %s: not allowed", node.Line, node.Op, v.Kind)
			}
			jumpTable = 0 // reset the jump table
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		// this section implements instructions that look like "OP VARIABLE FLAG(A|X)"
		case op.CAI, op.CAV:
			if minArgs := 2; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch v := node.Parameters[0]; v.Kind {
			case ast.Variable:
				symtab.AddReference(v.Text, machine.PC)
			default:
				return nil, fmt.Errorf("%d: %s: %s: not allowed", node.Line, node.Op, v.Kind)
			}
			switch flag := node.Parameters[1]; flag.Kind {
			case ast.Variable:
				switch flag.Text {
				case "A": // compare unsigned addresses
					// no special action needed
				case "X": // compare signed numbers
					// no special action needed
				default:
					return nil, fmt.Errorf("%d: %s: flag want A|X: got %q", node.Line, node.Op, flag)
				}
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, flag.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		// this section implements instructions that look like "OP VARIABLE FLAG(C|D)"
		case op.LAA:
			if minArgs := 2; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch v := node.Parameters[0]; v.Kind {
			case ast.Variable:
				symtab.AddReference(v.Text, machine.PC)
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, v.Kind)
			}
			switch flag := node.Parameters[1]; flag.Kind {
			case ast.Variable:
				switch flag.Text {
				case "C": // load A with the address of the table label
					// no special action needed
				case "D": // load A with the address of variable V
					// no special action needed
				default:
					return nil, fmt.Errorf("%d: %s: flag want C|D: got %q", node.Line, node.Op, flag.Text)
				}
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, flag.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		// this section implements instructions that look like "OP VARIABLE FLAG(P|X)"
		case op.STI, op.STV:
			if minArgs := 2; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch v := node.Parameters[0]; v.Kind {
			case ast.Variable:
				symtab.AddReference(v.Text, machine.PC)
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, v.Kind)
			}
			switch pxFlag := node.Parameters[1]; pxFlag.Kind {
			case ast.Variable:
				switch pxFlag.Text {
				case "P": // must preserve register A
					// no special action needed
				case "X": // okay to clobber register A
					// no special action needed
				default:
					return nil, fmt.Errorf("%d: %s: rxFlag want R|X: got %q", node.Line, node.Op, pxFlag.Text)
				}
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, pxFlag.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		// this section implements instructions that look like "OP VARIABLE FLAG(R|X)"
		case op.LAI, op.LAV, op.LCI:
			if minArgs := 2; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch v := node.Parameters[0]; v.Kind {
			case ast.Variable:
				symtab.AddReference(v.Text, machine.PC)
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, v.Kind)
			}
			switch flag := node.Parameters[1]; flag.Kind {
			case ast.Variable:
				switch flag.Text {
				case "R": // load may be redundant
					// no special action needed
				case "X": // load is not redundant
					// no special action needed
				default:
					return nil, fmt.Errorf("%d: %s: flag want R|X: got %q", node.Line, node.Op, flag.Text)
				}
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, flag.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		// this section implements instructions that look like "OP VARIABLE NUMBER"
		case op.IDENT:
			if minArgs := 2; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch v := node.Parameters[0]; v.Kind {
			case ast.Variable:
				// nothing special
			default:
				return nil, fmt.Errorf("%d: %s: want variable: got %s", node.Line, node.Op, v.Kind)
			}
			switch constant := node.Parameters[1]; constant.Kind {
			case ast.Number:
				symtab.InsertConstant(node.Line, node.Parameters[0].Text, constant.Number)
			default:
				return nil, fmt.Errorf("%d: %s: want constant: got %s", node.Line, node.Op, constant.Kind)
			}

		// this section implements instructions that look like "OP VARIABLE (NUMBER | CONSTANT_VAR | N-OF)"
		case op.BUMP:
			if minArgs := 2; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch v := node.Parameters[0]; v.Kind {
			case ast.Variable:
				symtab.AddReference(v.Text, machine.PC)
			default:
				return nil, fmt.Errorf("%d: %s: %s: not allowed", node.Line, node.Op, v.Kind)
			}
			switch nOF := node.Parameters[1]; nOF.Kind {
			case ast.Macro:
				if minArgs := 3; len(node.Parameters) < minArgs {
					return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
				}
				expr := node.Parameters[2]
				value, err := evalMacro(nOF.Text, expr, symtab.GetEnv())
				if err != nil {
					return nil, fmt.Errorf("%d: %s: %s %s: %w", node.Line, node.Op, nOF.Kind, nOF.Text, err)
				}
				word.ValueTwo = value
			case ast.Number:
				word.ValueTwo = nOF.Number
			case ast.Variable:
				// variable must be a constant
				sym, ok := symtab.Lookup(nOF.Text)
				if !ok {
					return nil, fmt.Errorf("%d: %s: %s %q: forward declaration not allowed here", node.Line, node.Op, nOF.Kind, nOF.Text)
				}
				switch sym.kind {
				case "constant":
					word.ValueTwo = sym.constant
				default:
					return nil, fmt.Errorf("%d: %s: %s: must be constant", node.Line, node.Op, nOF.Text)
				}
			default:
				return nil, fmt.Errorf("%d: %s: %s: not allowed", node.Line, node.Op, nOF.Kind)
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		// this section implements op codes that require a label spec
		case op.GO, op.GOEQ, op.GOGE, op.GOLE, op.GOLT, op.GOND, op.GONE, op.GOGR, op.GOPC:
			if minArgs := 4; len(node.Parameters) < minArgs {
				return nil, fmt.Errorf("%d: %s: want %d args: got %d", node.Line, node.Op, minArgs, len(node.Parameters))
			}
			switch label := node.Parameters[0]; label.Kind {
			case ast.Variable:
				symtab.AddReference(label.Text, machine.PC)
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, label.Kind)
			}
			switch distance := node.Parameters[1]; distance.Kind {
			case ast.Number:
				// no special action needed
			default:
				return nil, fmt.Errorf("%d: %s: %s not allowed", node.Line, node.Op, distance.Kind)
			}
			switch flag := node.Parameters[2]; flag.Kind {
			case ast.Variable:
				switch flag.Text {
				case "E": // branch out of subroutine
					// no special action needed
				case "X": // normal branch
					// no special action needed
				default:
					return nil, fmt.Errorf("%d: %s: flag wants E|X: got %q", node.Line, node.Op, flag.Text)
				}
			}
			switch flag := node.Parameters[3]; flag.Kind {
			case ast.Variable:
				switch flag.Text {
				case "C": // exit following gosub
					if node.Op != op.GO {
						return nil, fmt.Errorf("%d: %s: C: not allowed", node.Line, node.Op)
					}
					word.Op = op.GOTBL
					word.ValueTwo, jumpTable = jumpTable+1, jumpTable+1
				case "T": // GOADD branch
					if node.Op != op.GO {
						return nil, fmt.Errorf("%d: %s: T: not allowed", node.Line, node.Op)
					}
					word.Op = op.GOTBL
					word.ValueTwo, jumpTable = jumpTable, jumpTable+1
				case "X": // nothing special
					jumpTable = 0 // reset the jump table
				default:
					return nil, fmt.Errorf("%d: %s: flag wants C|T|X: got %q", node.Line, node.Op, flag.Text)
				}
			}
			machine.Core[machine.PC], machine.PC = word, machine.PC+1

		default:
			return nil, fmt.Errorf("%d: %s: not implemented", node.Line, node.Op)
		}
	}

	machine.Registers.Last = machine.PC

	// link the hash chains.
	//
	// Every name goes on the chain its hash picks out. The chain head, in the
	// block THASH reserved, holds the address of the first entry, and each
	// entry holds the address of the next, with zero ending the chain. That is
	// the shape the name search in ML/I walks: it loads the head, and while
	// the pointer is not zero it compares the entry one word further on and
	// then follows the pointer stored at the entry.
	//
	// Entries are threaded in reverse so that the chain comes out in source
	// order, which keeps the listing and the search agreeing with the source.
	if len(hashEntries) != 0 {
		if thashAddress == -1 {
			return nil, fmt.Errorf("%d: HASH: no THASH statement", hashEntries[0].line)
		}
		seen := make(map[string]int)
		for _, entry := range hashEntries {
			if line, ok := seen[entry.name]; ok {
				return nil, fmt.Errorf("%d: HASH: %q already hashed on line %d", entry.line, entry.name, line)
			}
			seen[entry.name] = entry.line
		}
		for i := len(hashEntries) - 1; i >= 0; i-- {
			entry := hashEntries[i]
			head := thashAddress + vm.HashName(entry.name, machine.Registers.LCH, machine.Registers.LNM)
			machine.Core[entry.address].Value = machine.Core[head].Value
			machine.Core[head].Value = entry.address
		}
	} else if thashAddress != -1 {
		return nil, fmt.Errorf("THASH: no HASH statements to build a table from")
	}

	// when we start running the machine, the PC should be set to the first
	// instruction in the program. if there is no BEGIN label, the PC will
	// point to a HALT instruction.
	if sym, ok := symtab.Lookup("BEGIN"); !ok {
		tracef("asm: warning: BEGIN not set\n")
	} else {
		if sym.kind != "address" {
			panic("BEGIN must be a label")
		}
		tracef("asm: set vm begin   %-12s %6d\n", "", sym.address)
		machine.Registers.Start = sym.address
	}

	// detect and report undefined symbols
	undefinedSymbols := 0
	for _, sym := range symtab.symbols {
		if sym.kind != "undefined" && sym.line != 0 {
			continue
		}
		tracef("asm: error: undefined symbol %q %q\n", sym.name, sym.kind)
		undefinedSymbols++
	}
	if undefinedSymbols != 0 {
		return nil, fmt.Errorf("found %d undefined symbols", undefinedSymbols)
	}

	// back-fill as needed
	//
	// fill stores one symbol's value in one waiting word. A relative reference
	// wants the distance to the symbol instead of the symbol itself, and the
	// distance the source claimed is checked against the one that was laid
	// out: the tables are walked by adding a word to the address it came from,
	// so a table whose items have drifted apart is not a wrong number, it is a
	// pointer into the middle of a string.
	fill := func(name string, value int, ref backFillRef) error {
		if !ref.relative {
			machine.Core[ref.address].Value = value + ref.offset
			return nil
		}
		distance := value - ref.address
		if distance != ref.offset {
			return fmt.Errorf("%d: RL %s: source says %d words away: assembled %d", machine.Core[ref.address].Source.Line, name, ref.offset, distance)
		}
		machine.Core[ref.address].Value = distance
		return nil
	}
	for _, sym := range symtab.symbols {
		value := 0
		switch sym.kind {
		case "address":
			value = sym.address
		case "alias":
			aliasOf, ok := symtab.Lookup(sym.alias)
			if !ok {
				return nil, fmt.Errorf("alias %q never defined", sym.name)
			}
			switch aliasOf.kind {
			case "address":
				value = aliasOf.address
			case "constant":
				value = aliasOf.constant
			default:
				panic(fmt.Sprintf("assert(aliasOf.kind != %q)", aliasOf.kind))
			}
		case "constant":
			value = sym.constant
		default:
			panic(fmt.Sprintf("assert(sym.kind != %q)", sym.kind))
		}
		for _, ref := range sym.backFill {
			if err := fill(sym.name, value, ref); err != nil {
				return nil, err
			}
		}
	}

	// hand the machine every name that resolved to an address. The machine
	// dependent subroutines are written against the variables of the program
	// they serve, and this is how they find them; aliases are included
	// because two of those variables, OP1 and OPSW, are EQU'd on to others.
	for _, sym := range symtab.symbols {
		if resolved, ok := symtab.Lookup(sym.name); ok && resolved.kind == "address" {
			machine.Symbols[sym.name] = resolved.address
		}
	}

	// dump the symbol table
	fpListing := &bytes.Buffer{}
	var list []string
	for _, sym := range symtab.symbols {
		list = append(list, sym.name)
	}
	sort.Strings(list)
	for _, name := range list {
		sym := symtab.symbols[name]
		var dfd, dfl string
		switch {
		case sym.line < 0:
			dfd, dfl = " ", "****"
		case sym.line == 0:
			dfd, dfl = "*", "****"
		default:
			dfd, dfl = " ", fmt.Sprintf("%4d", sym.line)
		}
		switch sym.kind {
		case "address":
			_, _ = fmt.Fprintf(fpListing, "%s %-12s %-8s %8d  %s\n", dfd, sym.name, sym.kind, sym.address, dfl)
		case "alias":
			_, _ = fmt.Fprintf(fpListing, "%s %-12s %-8s %-8s  %s\n", dfd, sym.name, sym.kind, sym.alias, dfl)
		case "constant":
			_, _ = fmt.Fprintf(fpListing, "%s %-12s %-8s %8d  %s\n", dfd, sym.name, sym.kind, sym.constant, dfl)
		default:
			_, _ = fmt.Fprintf(fpListing, "%s %-12s %-8s %8d  %s\n", dfd, sym.name, sym.kind, -6_666, dfl)
		}
	}

	if opts.Listings {
		_ = os.WriteFile("asm_symtab.txt", fpListing.Bytes(), 0644)
		if err := Listing("asm_listing.txt", machine, symtab); err != nil {
			return nil, err
		}
	}

	return machine, nil
}
