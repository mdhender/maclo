// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ast

import "github.com/mdhender/maclo/pkg/l/stmt"

// Every statement reports which statement of L it is, so that a consumer that
// only needs the name - a diagnostic, a listing header, the check that a
// statement is legal in its SECTION - does not need a type switch of its own.
// The mapping is here rather than spread through stmts.go so that the whole of
// it can be read at once against the table in pkg/l/stmt.

func (s *PrgStart) Kind() stmt.Kind     { return stmt.PrgStart }
func (s *PrgEnd) Kind() stmt.Kind       { return stmt.PrgEnd }
func (s *Section) Kind() stmt.Kind      { return stmt.Section }
func (s *Dec) Kind() stmt.Kind          { return stmt.Dec }
func (s *Equate) Kind() stmt.Kind       { return stmt.Equate }
func (s *BlockDec) Kind() stmt.Kind     { return stmt.BlockDec }
func (s *Subroutine) Kind() stmt.Kind   { return stmt.Subroutine }
func (s *LinkRoutine) Kind() stmt.Kind  { return stmt.LinkRoutine }
func (s *ReturnFrom) Kind() stmt.Kind   { return stmt.ReturnFrom }
func (s *ExitFrom) Kind() stmt.Kind     { return stmt.ExitFrom }
func (s *LinkBack) Kind() stmt.Kind     { return stmt.LinkBack }
func (s *Call) Kind() stmt.Kind         { return stmt.Call }
func (s *If) Kind() stmt.Kind           { return stmt.If }
func (s *ChainFrom) Kind() stmt.Kind    { return stmt.ChainFrom }
func (s *MoveFrom) Kind() stmt.Kind     { return stmt.MoveFrom }
func (s *MStackFrom) Kind() stmt.Kind   { return stmt.MStackFrom }
func (s *MUnstackFrom) Kind() stmt.Kind { return stmt.MUnstackFrom }
func (s *Read) Kind() stmt.Kind         { return stmt.Read }
func (s *OutputID) Kind() stmt.Kind     { return stmt.OutputID }
func (s *PRText) Kind() stmt.Kind       { return stmt.PRText }
func (s *Backspace) Kind() stmt.Kind    { return stmt.Backspace }
func (s *CharMatch) Kind() stmt.Kind    { return stmt.CharMatch }
func (s *GoTo) Kind() stmt.Kind         { return stmt.GoTo }
func (s *Scale) Kind() stmt.Kind        { return stmt.Scale }
func (s *Set) Kind() stmt.Kind          { return stmt.Set }
func (s *SetSW) Kind() stmt.Kind        { return stmt.SetSW }
func (s *Stack) Kind() stmt.Kind        { return stmt.Stack }
func (s *Unstack) Kind() stmt.Kind      { return stmt.Unstack }
func (s *Test) Kind() stmt.Kind         { return stmt.Test }
func (s *DC) Kind() stmt.Kind           { return stmt.DC }
func (s *LayChain) Kind() stmt.Kind     { return stmt.LayChain }
func (s *HETables) Kind() stmt.Kind     { return stmt.HETables }
func (s *OpMac) Kind() stmt.Kind        { return stmt.OpMac }

// BadStmt is the one statement with no kind: it stands for a line that could
// not be read as any of them.
func (s *BadStmt) Kind() stmt.Kind { return stmt.Unknown }
