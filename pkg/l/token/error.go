// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package token

import (
	"fmt"
	"sort"
	"strings"
)

// Stage names the pass that found a problem. It is on the error because the
// front end accumulates rather than bailing: a report over a 2,500 line file
// is easier to read when "the scanner could not make a token of this" and
// "nothing defines this name" are distinguishable at a glance.
type Stage uint8

// enums for Stage
const (
	StageScanner Stage = iota
	StageCST
	StageAST
	StageSema
)

// String implements the Stringer interface.
func (s Stage) String() string {
	switch s {
	case StageScanner:
		return "scan"
	case StageCST:
		return "parse"
	case StageAST:
		return "build"
	case StageSema:
		return "check"
	}
	return fmt.Sprintf("Stage(%d)", int(s))
}

// Severity separates the things that make a source file wrong from the things
// that only make it questionable. The manual's three-to-six character rule for
// identifiers is the motivating case: the corpus breaks it (STOPCODE is eight,
// and so are two SECTION names), so enforcing it as an error would reject real
// L, and dropping it entirely would lose a real check.
type Severity uint8

// enums for Severity
const (
	SevError Severity = iota
	SevWarning
)

// String implements the Stringer interface.
func (s Severity) String() string {
	switch s {
	case SevError:
		return "error"
	case SevWarning:
		return "warning"
	}
	return fmt.Sprintf("Severity(%d)", int(s))
}

// Error is one diagnostic. It is a value rather than an error interface
// because every stage collects them into one list and the list is sorted by
// position before it is reported.
type Error struct {
	Pos      Position
	Stage    Stage
	Severity Severity
	Msg      string
}

// Error implements the error interface.
func (e Error) Error() string {
	return fmt.Sprintf("%s: %s: %s: %s", e.Pos, e.Severity, e.Stage, e.Msg)
}

// Errors is an accumulated diagnostic list.
//
// Every stage of this front end takes one of these and keeps going, which is
// the opposite of what pkg/lowl/ast and pkg/lowl/assembler do. The reason is
// the deliverable: a listing of a 2,510 line file that you read. Stopping at
// the first problem means one typo fixed per run, and the L source of ML/I has
// a real defect near the top of its main section, so a bail-on-first front end
// would hide the 2,150 lines after it.
type Errors []Error

// Add appends an error at pos.
func (e *Errors) Add(pos Position, stage Stage, format string, args ...any) {
	*e = append(*e, Error{Pos: pos, Stage: stage, Severity: SevError, Msg: fmt.Sprintf(format, args...)})
}

// Warn appends a warning at pos.
func (e *Errors) Warn(pos Position, stage Stage, format string, args ...any) {
	*e = append(*e, Error{Pos: pos, Stage: stage, Severity: SevWarning, Msg: fmt.Sprintf(format, args...)})
}

// Merge appends every diagnostic in other.
func (e *Errors) Merge(other Errors) {
	*e = append(*e, other...)
}

// HasErrors reports whether the list holds anything of severity SevError.
// A list of nothing but warnings is not a failure.
func (e Errors) HasErrors() bool {
	for _, err := range e {
		if err.Severity == SevError {
			return true
		}
	}
	return false
}

// Sorted returns the diagnostics in source order. Ties keep the order they
// were added in, so a stage that reports twice about one place reads in the
// order it found things.
func (e Errors) Sorted() Errors {
	out := make(Errors, len(e))
	copy(out, e)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Pos.Before(out[j].Pos)
	})
	return out
}

// Error implements the error interface, so a whole list can be returned as
// one error when a caller wants that rather than the individual diagnostics.
func (e Errors) Error() string {
	if len(e) == 0 {
		return "no errors"
	}
	var sb strings.Builder
	for i, err := range e.Sorted() {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(err.Error())
	}
	return sb.String()
}
