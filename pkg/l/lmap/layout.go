// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package lmap

import (
	"strconv"

	"github.com/mdhender/maclo/pkg/l/token"
	"github.com/mdhender/maclo/pkg/lowl/vm"
)

// Where the words go, and why anyone has to know.
//
// RL is the reason. A delimiter table is walked by adding the word just read
// to the address it was read from, so an RL item holds a distance rather than
// an address, and the distance is written into the source. pkg/lowl/assembler
// checks the distance the source claims against the one it laid out and
// refuses the program when the two disagree -- a table whose items have
// drifted apart is not a wrong number, it is a pointer into the middle of a
// string, so it is worth catching at assembly time.
//
// That means the L-map has to know the size of every word it emits before it
// can write the first RL. It also means the check is free: get a length wrong
// anywhere in the data SECTIONs and the assembler names the line.
//
// The distance is kept as a pair rather than a count. LOWL's OF macro is how a
// machine independent length is written, and a delimiter item is part numbers
// and part characters, so "four numbers and four characters further on" is
// OF(4*LNM+4*LCH) and stays right on a machine where the two differ. This
// machine has LNM and LCH both one, so the pair adds up to the word count and
// the assembler's check is on the total; keeping the parts apart costs
// nothing and is the form the published LOWL is written in.

// offset is a position in the program, in the units OF is written in.
type offset struct {
	nm int // words holding a number, a pointer or an instruction
	ch int // words holding a character
}

func (o offset) sub(b offset) offset { return offset{nm: o.nm - b.nm, ch: o.ch - b.ch} }
func (o offset) words() int          { return o.nm + o.ch }

// String renders the offset as the argument of an OF macro.
//
// pkg/postfix has no unary minus -- a leading operator underflows its operand
// stack -- so a distance with a negative part is written as a subtraction from
// zero instead. Nothing in the data SECTIONs of ML/I chains backwards, so this
// is a guard rather than a path anything takes.
func (o offset) String() string {
	if o.nm >= 0 && o.ch >= 0 {
		return strconv.Itoa(o.nm) + "*LNM+" + strconv.Itoa(o.ch) + "*LCH"
	}
	if n := o.words(); n >= 0 {
		return strconv.Itoa(n) + "*LNM"
	} else {
		return "0*LNM-" + strconv.Itoa(-n) + "*LNM"
	}
}

// size is how many words a statement puts in core, split the way OF splits
// them. It has to agree with pkg/lowl/assembler, and the RL check is what
// says whether it does.
func (s Stmt) size() offset {
	switch s.Op {
	case "":
		// a label or a blank line
		return offset{}
	case "PRGST", "NB", "EQU", "IDENT", "ALIGN":
		// the statements the assembler reads and emits nothing for
		return offset{}
	case "STR":
		// one word per character
		if len(s.Args) == 0 {
			return offset{}
		}
		return offset{ch: len([]rune(s.Args[0].Text))}
	case "NCH":
		return offset{ch: 1}
	case "THASH":
		// one word per hash chain head
		return offset{nm: vm.LHV}
	}
	return offset{nm: 1}
}

// layout walks the program, works out where every label is, and fills in the
// distance each RL claims.
func (p *Program) layout(errs *token.Errors) {
	at := make(map[string]offset)
	var pos offset
	for _, s := range p.Stmts {
		if s.Op == "" && s.Label != "" {
			if _, seen := at[s.Label]; seen {
				errs.Add(token.Position{}, token.StageLMap, "%s is defined twice in the generated LOWL", s.Label)
			}
			at[s.Label] = pos
		}
		sz := s.size()
		pos.nm += sz.nm
		pos.ch += sz.ch
	}

	// a second walk, because a fixup may measure to a label that comes after
	// the RL and there is no useful order to do both in
	pos = offset{}
	for i, s := range p.Stmts {
		if len(p.fixups) != 0 && p.fixups[0].stmt == i {
			f := p.fixups[0]
			p.fixups = p.fixups[1:]
			target, ok := at[f.target]
			if !ok {
				errs.Add(token.Position{Line: s.Line}, token.StageLMap, "RL %s: nothing in the generated LOWL carries that label", f.target)
			} else {
				d := target.sub(pos)
				d.nm += f.adjust
				p.Stmts[i].Args[1] = ofArg(d.String())
			}
		}
		sz := s.size()
		pos.nm += sz.nm
		pos.ch += sz.ch
	}
}
