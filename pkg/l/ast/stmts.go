// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ast

import "github.com/mdhender/maclo/pkg/l/token"

// The closing statements of L - ENDSECT, ENDBLOCK, ENDSUB, END and ENDCH -
// have no types here. build.go consumes each one and folds its position, its
// name and any label it carried onto the statement it closes. They are
// punctuation for a nested tree rather than statements in it, which is the
// whole point of building a tree instead of a list.

// --- layout (lmap.txt 2.4) -------------------------------------------------

// PrgStart is the first statement of the logic.
type PrgStart struct{ Base }

// PrgEnd is the last.
type PrgEnd struct{ Base }

// Section is one of the ten pieces the logic is divided into.
type Section struct {
	Base
	Name *Ident
	// Comment is the subsidiary comment after the comma, without its
	// delimiters. It names the SECTION for a reader and nothing else.
	Comment string
	Body    []Stmt
	// EndPos and EndName come from the ENDSECT. The name is kept rather than
	// checked here so sema can report the mismatch with both positions.
	EndPos   token.Position
	EndName  *Ident
	EndLabel *Ident
}

// --- declarative (lmap.txt 5) ----------------------------------------------

// Dec declares a variable. Init is the INIT value, or nil.
//
// Both this and the INVALS SECTION carry initial values, and an L-map uses one
// or the other depending on whether it initialises statically or dynamically
// (lmap.txt 5.1). Neither is authoritative, so both are kept.
type Dec struct {
	Base
	Name *Ident
	Init Expr
}

// Equate makes one name another name for the same variable.
type Equate struct {
	Base
	Name *Ident
	To   *Ident
}

// BlockDec groups variables that are moved about as a unit by the block moving
// statements. Corresponding to each is a constant naming its size, which is
// the block's name with SZ appended.
type BlockDec struct {
	Base
	Name     *Ident
	Comment  string
	Body     []Stmt // Dec, and in one case in the logic of ML/I a nested BlockDec
	EndPos   token.Position
	EndName  *Ident
	EndLabel *Ident
}

// --- routines (lmap.txt 4.1) -----------------------------------------------

// ParamSpec is the (PARPT), (PARNM) or (PARSW) on a subroutine.
type ParamSpec struct {
	Position token.Position
	Name     string // PARPT, PARNM or PARSW
	Type     DataType
}

// Pos implements Node.
func (p *ParamSpec) Pos() token.Position { return p.Position }

// Subroutine is a subroutine and its body.
//
// HasExit is a bool rather than a count. L allows a subroutine at most one
// exit label - the grammar is singular (lmap.txt 4.1.1.1) and the logic of
// ML/I never declares two. This is where L and LOWL part company: LOWL's SUBR
// records an exit count and the assembler builds a jump table for it, and none
// of that belongs here.
type Subroutine struct {
	Base
	Name    *Ident
	Param   *ParamSpec // nil when the subroutine takes no argument
	HasExit bool
	ExitPos token.Position
	// ExitComment describes what the exit means. It is for the reader.
	ExitComment string
	Body        []Stmt
	EndPos      token.Position
	EndLabel    *Ident
}

// LinkRoutine is the linkroutine (lmap.txt 4.1.2.1). There is one in the logic
// of ML/I, and it closes with ENDSUB like a subroutine, which is why the file
// holds one more ENDSUB than SUBROUTINE.
type LinkRoutine struct {
	Base
	Name     *Ident
	Body     []Stmt
	EndPos   token.Position
	EndLabel *Ident
}

// ReturnFrom returns from the subroutine it names, which must be the enclosing
// one.
type ReturnFrom struct {
	Base
	Name *Ident
}

// ExitFrom leaves by the subroutine's exit rather than its normal return.
type ExitFrom struct {
	Base
	Name *Ident
}

// LinkBack returns from the linkroutine.
type LinkBack struct{ Base }

// CallArg is the parenthesised argument of a CALL and its mandatory type.
type CallArg struct {
	Position token.Position
	Value    Expr
	Type     DataType
}

// Pos implements Node.
func (a *CallArg) Pos() token.Position { return a.Position }

// Call calls a subroutine, a linkroutine or a machine-dependent subroutine.
//
// The manual gives three forms - no argument, an arithmetic argument, and a
// switch argument (lmap.txt 4.1.3) - and they collapse into one struct because
// they differ in the type of the value rather than the shape of the statement.
// This pass does not type check, so the distinction has nowhere to go.
type Call struct {
	Base
	Name *Ident
	Arg  *CallArg // nil for the form with no argument
	Exit *Ident   // the EXIT label, or nil
}

// --- compound (lmap.txt 4.2) -----------------------------------------------

// Rel is one comparison.
type Rel struct {
	Position token.Position
	X        Expr
	Op       RelOp
	Y        Expr
}

// Pos implements Node.
func (r *Rel) Pos() token.Position { return r.Position }

// Cond is the condition of an IF: one relation, or several joined by all-&
// or all-|.
//
// The join is one operator for the whole condition rather than a tree, because
// the two cannot be mixed (lmap.txt 4.2.1.1). Encoding it this way makes the
// rule unrepresentable instead of merely unchecked.
type Cond struct {
	Position token.Position
	Rels     []*Rel
	Join     BinOp // And or Or; meaningless when there is one relation
}

// Pos implements Node.
func (c *Cond) Pos() token.Position { return c.Position }

// If is both forms of the conditional (lmap.txt 4.2.1): the one-line form,
// where a single statement follows THEN, and the block form, where THEN ends
// the line and END closes it.
//
// One type covers both. The manual makes something of the difference - it
// calls the newline of the one-line form a closing delimiter of two statements
// at once - but a consumer almost always wants "the condition and what it
// guards", and two types would double every type switch to no purpose. Block
// is set from one fact: whether anything followed THEN on the line.
type If struct {
	Base
	Cond  *Cond
	Block bool
	// Then is the guarded statement of the one-line form; nil in the block
	// form. Use Stmts to read either.
	Then Stmt
	// Body is the guarded statements of the block form.
	Body     []Stmt
	EndPos   token.Position
	EndLabel *Ident
}

// Stmts returns the statements the condition guards, whichever form was used.
func (s *If) Stmts() []Stmt {
	if s.Block {
		return s.Body
	}
	if s.Then == nil {
		return nil
	}
	return []Stmt{s.Then}
}

// ChainFrom walks a chain of blocks (lmap.txt 4.2.2).
type ChainFrom struct {
	Base
	Addr Expr
	Exit *Ident // mandatory: where to go when the chain is exhausted
	Body []Stmt
	// EndPos and EndLabel come from the ENDCH.
	//
	// EndLabel matters more than it looks. The ENDCH may be labelled, and four
	// of the eleven chains in the logic of ML/I label theirs; one of those
	// labels is the exit of a CALL three lines above it. Dropping the label
	// when the closer is folded away would make four defined labels vanish and
	// four uses of them undefined.
	EndPos   token.Position
	EndLabel *Ident
}

// ImplicitVars names the two variables a CHAIN FROM reads and writes without
// mentioning them: CHANPT, which points at the current block, and CHLINK,
// which holds the link to the next (lmap.txt 4.2.2).
//
// This front end does not desugar the loop. It does register a use of both, so
// that they are neither reported as unused nor silently allowed to be
// undeclared.
func (s *ChainFrom) ImplicitVars() []string { return []string{"CHANPT", "CHLINK"} }

// --- block moves (lmap.txt 4.3) --------------------------------------------

// MoveFrom copies a block of storage. From and To may be a BlockRef.
type MoveFrom struct {
	Base
	From      Expr
	To        Expr
	Leng      Expr
	Backwards bool
}

// MStackFrom copies a block onto one of the stacks.
type MStackFrom struct {
	Base
	From Expr
	Leng Expr
	On   StackName
}

// MUnstackFrom copies a block off the backwards stack. The grammar admits no
// other stack, so which one is not stored.
type MUnstackFrom struct {
	Base
	From Expr
	To   Expr
	Leng Expr
}

// --- input and output (lmap.txt 4.4) ---------------------------------------

// Read reads the next character of the source text.
type Read struct{ Base }

// OutputID copies the current atom to the output.
type OutputID struct{ Base }

// PRText writes a fixed string to the debugging output.
type PRText struct {
	Base
	Text *TextLit
}

// --- assignment and branching (lmap.txt 4.5) -------------------------------

// Backspace reads a variable's former value from the backwards stack. Giving
// is nil in the form that leaves the address in TEMPT.
type Backspace struct {
	Base
	Var    *Ident
	Giving *Ident
}

// CharArm is one arm of a CHARMATCH: a character and where to go on it.
type CharArm struct {
	Position token.Position
	Char     Expr
	Target   *Ident
}

// Pos implements Node.
func (a *CharArm) Pos() token.Position { return a.Position }

// CharMatch branches on the character a pointer addresses.
type CharMatch struct {
	Base
	Ptr  *Ident
	Arms []*CharArm
}

// GoTo is the unconditional branch.
type GoTo struct {
	Base
	Target *Ident
}

// Scale multiplies a number by a constant. Giving is nil when the result
// replaces the operand.
type Scale struct {
	Base
	Var    *Ident
	By     Expr
	Giving *Ident
}

// Set assigns an arithmetic value. Targets may be Idents or Inds, and there
// may be several.
type Set struct {
	Base
	Targets []Expr
	Value   Expr
}

// SetSW assigns a switch. The form that ands or ors two operands arrives here
// as a Binary, which keeps Set and SetSW the same shape for a walker.
type SetSW struct {
	Base
	Targets []Expr
	Value   Expr
}

// StackVal is one value being stacked or unstacked, with its type tag.
//
// The tag is written after the value and there is no comma between values:
// "STACK IDPT (PT) BESPT+OF(LCH) (PT) ON BSTACK". The space before the tag is
// optional, so the parser cannot use it to find the boundary.
type StackVal struct {
	Position token.Position
	Value    Expr
	Type     DataType
}

// Pos implements Node.
func (v *StackVal) Pos() token.Position { return v.Position }

// Stack pushes values onto one of the stacks.
type Stack struct {
	Base
	Values []*StackVal
	On     StackName
}

// Unstack pops values off the backwards stack, which is the only one it may
// name.
type Unstack struct {
	Base
	Values []*StackVal
}

// Test branches on a small number: value 1 goes to the first label, 2 to the
// second, and so on.
type Test struct {
	Base
	Var     *Ident
	Targets []*Ident
}

// --- the data SECTIONs (lmap.txt 6) ----------------------------------------

// DC lays down constants. Its arguments are RL, LID, quotes, integers and
// named constants; the logic of ML/I also uses OF.
type DC struct {
	Base
	Args []Expr
}

// LayChain lays down the chain of delimiter blocks.
//
// It can carry a label, and in the logic of ML/I it does - which contradicts
// the manual's claim that a data label always precedes a DC.
type LayChain struct{ Base }

// HETables lays down the hash and error tables. It has no arguments, and it
// defines three data labels that appear nowhere else in the source: ERBLOC,
// GHSHTB and SVEC.
type HETables struct{ Base }

// OpMac describes one operation macro.
//
// The fields are named for what the manual's action says they do rather than
// for their positions, because arg1, arg2 and arg3 would be unreadable
// (lmap.txt 6.2.4). Name2 is the second atom of the "+" form, as in
// OPMAC 'MCSUB'+'(', and is nil when there is none.
type OpMac struct {
	Base
	Name   *CharLit
	Name2  *CharLit
	Dels   Expr // the delimiter chain, RL(...) or ENDCHN
	Marker Expr // OPMK or LOCMK
	Number Expr // the operation macro's number
}

// BadStmt is a line the builder could not turn into a statement. It keeps the
// line in the tree so that recovery is per statement and the listing shows
// where the hole is.
type BadStmt struct {
	Base
	Raw string
}
