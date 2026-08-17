// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ast

import "github.com/mdhender/maclo/pkg/l/token"

// Ident is any occurrence of a name: a variable being declared or read, a
// label being defined or branched to, a subroutine being called, a SECTION
// being opened or closed.
//
// There is one type rather than a Var, a NamedConst, a LenName and a LabelRef
// because the difference between them is what a name resolves to, not how it
// is written. Deciding at parse time would mean this package holding the list
// of predefined constants, which belongs to sema. So build produces Idents and
// sema says what each one is.
type Ident struct {
	Position token.Position
	Text     string
}

// Pos implements Node.
func (i *Ident) Pos() token.Position { return i.Position }
func (i *Ident) exprNode()           {}

// IntLit is an integer constant. Most constants in L are single digits
// (lmap.txt 3.3), but the hash-table size and the operation macro numbers are
// not.
type IntLit struct {
	Position token.Position
	Value    int
	Raw      string
}

// Pos implements Node.
func (l *IntLit) Pos() token.Position { return l.Position }
func (l *IntLit) exprNode()           {}

// CharLit is the quote macro (lmap.txt 3.3.2). Its argument is a single
// character in the program SECTIONs and any atom in the data SECTIONs, which
// is why Text is a string. Raw is what was written and Text has $ decoded to a
// newline; the listing wants the first and a back end the second.
type CharLit struct {
	Position token.Position
	Raw      string
	Text     string
}

// Pos implements Node.
func (l *CharLit) Pos() token.Position { return l.Position }
func (l *CharLit) exprNode()           {}

// TextLit is a bracketed run of characters: the argument of PRTEXT
// (lmap.txt 4.4.3) or of LID (lmap.txt 6.1.2). Spaces inside it are
// significant.
type TextLit struct {
	Position token.Position
	Raw      string
	Text     string
}

// Pos implements Node.
func (l *TextLit) Pos() token.Position { return l.Position }
func (l *TextLit) exprNode()           {}

// OF is the storage-size macro, OF(2*LPT - LSW) (lmap.txt 3.3.1). Its argument
// is the one place multiplication and division may appear.
type OF struct {
	Position token.Position
	Arg      Expr
}

// Pos implements Node.
func (e *OF) Pos() token.Position { return e.Position }
func (e *OF) exprNode()           {}

// AD is the address-of macro, AD(KSPACE)PT (lmap.txt 3.3.3). Its argument is
// always a data label and its type is always PT, so neither is stored.
type AD struct {
	Position token.Position
	Name     *Ident
}

// Pos implements Node.
func (e *AD) Pos() token.Position { return e.Position }
func (e *AD) exprNode()           {}

// BlockRef is the BLOCK macro, which names a group of variables declared with
// BLOCKDEC. It appears only in the block moving statements (lmap.txt 4.3).
type BlockRef struct {
	Position token.Position
	Name     *Ident
}

// Pos implements Node.
func (e *BlockRef) Pos() token.Position { return e.Position }
func (e *BlockRef) exprNode()           {}

// Ind is an indirect address, IND(HASHPT + TEMP - OF(LCH))SW
// (lmap.txt 3.4). The type suffix is mandatory and all four occur, so unlike
// AD it is kept.
type Ind struct {
	Position token.Position
	Addr     Expr
	Type     DataType
}

// Pos implements Node.
func (e *Ind) Pos() token.Position { return e.Position }
func (e *Ind) exprNode()           {}

// RL is the relative-location macro, RL(DVARS) or RL(D+OF(LNM))
// (lmap.txt 6.1.1). It appears only as an argument of DC or OPMAC.
type RL struct {
	Position token.Position
	Name     *Ident
	// Adjust is the signed offset written after the label, or nil.
	Adjust Expr
}

// Pos implements Node.
func (e *RL) Pos() token.Position { return e.Position }
func (e *RL) exprNode()           {}

// LID is the length-and-identifier macro, LID[SPACES] (lmap.txt 6.1.2). Like
// RL it appears only in the data SECTIONs.
type LID struct {
	Position token.Position
	Text     *TextLit
}

// Pos implements Node.
func (e *LID) Pos() token.Position { return e.Position }
func (e *LID) exprNode()           {}

// Binary is an operation on two values.
type Binary struct {
	Position token.Position
	Op       BinOp
	X, Y     Expr
}

// Pos implements Node.
func (e *Binary) Pos() token.Position { return e.Position }
func (e *Binary) exprNode()           {}

// Unary is a leading sign, as in "-6" or "- SKVAL + OF(LPT)".
type Unary struct {
	Position token.Position
	Op       BinOp
	X        Expr
}

// Pos implements Node.
func (e *Unary) Pos() token.Position { return e.Position }
func (e *Unary) exprNode()           {}

// Bad stands in for an expression the builder could not make sense of. It
// keeps a malformed statement in the tree, with its position, so one bad
// argument costs one diagnostic instead of discarding the statement around it.
type Bad struct {
	Position token.Position
	Raw      string
}

// Pos implements Node.
func (e *Bad) Pos() token.Position { return e.Position }
func (e *Bad) exprNode()           {}
