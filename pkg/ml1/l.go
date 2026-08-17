// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ml1

import (
	"bytes"
	"fmt"
	"os"

	"github.com/mdhender/maclo/pkg/l"
	"github.com/mdhender/maclo/pkg/l/lmap"
)

// The L backend.
//
// ML/I is distributed twice. The LOWL source pkg/ml1/lowl.go runs is what an
// L-map produced in 1971 from the other one, which is L: the machine
// independent language the logic is actually written in. This back end starts
// from that instead, maps it into LOWL, and hands the result to the same
// assembler.
//
// The point is not that it is a better way to get a processor -- the LOWL is
// on disk already and needs no translation -- but that it is the way a machine
// with no LOWL implementation would have to be reached, and that having both
// makes each a check on the other. pkg/ml1/l_test.go runs the whole local
// corpus through both and requires them to agree.
//
// Nothing is written to a file on the way. lmap.Program renders into a buffer
// and the buffer is assembled, so a generated engine never exists as a file
// unless a command was asked to write one. That matters beyond tidiness: the
// generated LOWL is a derivative of a source whose licence forbids
// redistributing a machine readable copy, so it must not be able to end up
// somewhere it would be committed from.

// runL performs the job on a machine mapped from the L source.
func runL(job Job) (Result, error) {
	src, err := os.ReadFile(job.LSource)
	if err != nil {
		return Result{Fatal: true}, fmt.Errorf("%s: %w", job.LSource, ErrNoEngineSource)
	}

	source, err := MapL(src)
	if err != nil {
		return Result{Fatal: true}, err
	}

	m, err := assemble(source)
	if err != nil {
		return Result{Fatal: true}, err
	}
	return runMachine(job, m)
}

// MapL translates the L source of ML/I into LOWL.
//
// It is exported because it is the only part of the L back end a caller might
// want without a process to run: cmd/macl can show the LOWL a program maps
// into, and a test can assemble it without driving it.
//
// A program that does not resolve is refused rather than translated. The front
// end reports as much as it can and keeps going, which is right for a listing
// and wrong for an engine: a branch to a label nothing declares maps into a
// branch to a label nothing defines, and the assembler would report that as a
// fault of the generated text rather than of the source it came from.
func MapL(src []byte) ([]byte, error) {
	result := l.Parse(src)
	if result.HasErrors() {
		return nil, fmt.Errorf("%w:\n%s", ErrLSourceErrors, result.Errs.Sorted().Error())
	}

	prog, errs := lmap.Map(result.Program, result.Table)
	if errs.HasErrors() {
		return nil, fmt.Errorf("%w:\n%s", ErrLSourceErrors, errs.Sorted().Error())
	}

	var buf bytes.Buffer
	if err := prog.WriteLOWL(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
