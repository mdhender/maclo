// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mdhender/maclo/pkg/lowl/assembler"
	"github.com/mdhender/maclo/pkg/lowl/ast"
	"github.com/mdhender/maclo/pkg/lowl/cst"
	"github.com/mdhender/maclo/pkg/lowl/vm"
)

// The LOWL backend.
//
// ML/I is distributed as source written in LOWL, and this runs that source
// rather than a translation of it: pkg/lowl assembles the .lwl into a machine
// and this file supplies the machine dependent subroutines, the storage, and
// the S-variables, then drives it.
//
// The source cannot be committed here — its licence forbids redistributing a
// machine readable copy — so it is loaded from a path at run time. engine.go
// has the search order and why it is what it is.

// runCycleLimit is a runaway guard, not a budget. Real input runs for tens of
// millions of instructions, so this is set where a stuck machine is caught in
// a few seconds and a working one never notices.
const runCycleLimit = 500_000_000

// runLOWL performs the job on a machine assembled from the LOWL source.
func runLOWL(job Job) (Result, error) {
	source, err := readSource(job.LOWLSource, job.Engine)
	if err != nil {
		return Result{Fatal: true}, err
	}

	m, err := assemble(source)
	if err != nil {
		return Result{Fatal: true}, err
	}
	return runMachine(job, m)
}

// runMachine drives an assembled machine, whichever back end built it.
//
// Everything below this line is the same for both: the storage, the
// S-variables, the machine dependent subroutines the host supplies, and the
// reading back of what the process did. A back end's whole job is to produce
// the machine.
func runMachine(job Job, m *vm.VM) (Result, error) {
	// nothing has been written yet, so the output text is at the start of a
	// line: the first character to arrive steps S19 and, if a listing was asked
	// for, carries the number of line one in front of it.
	h := &host{job: job, m: m, atLineStart: true}
	for _, in := range job.Inputs {
		h.streams = append(h.streams, &stream{in: in})
	}
	defer func() {
		for _, s := range h.streams {
			s.close()
		}
	}()

	// the storage has to exist before the first instruction runs: the
	// S-variables first, because ML/I reads them and never builds them, and
	// then the workspace behind them.
	next, err := m.SetSystemVariables(0, systemVariables())
	if err != nil {
		return Result{Fatal: true}, err
	}
	m.SetWorkspace(next, job.Workspace)
	h.svarpt = m.Core[m.Symbols["SVARPT"]].Value
	m.Host = h

	runErr := drive(m, h)
	res := result(m, h)
	if runErr != nil {
		return res, runErr
	}
	if res.Errors != 0 {
		return res, ErrProcessErrors
	}
	return res, nil
}

// drive runs the machine, watching for the two places where the end of process
// report begins. The first of them is also where a process that was aborted
// says so, which is why the host is told rather than assigned to.
//
// It is a loop of its own rather than vm.Run because of that watch. The LOWL
// MI-logic writes the whole report unconditionally — S18 is a much later
// addition, and the LOWL source has not been updated since 1986 — so the only
// way to honour S18 is to know which part of the report is being written. The
// statistics start when control reaches the finalisation code and the list of
// constructions starts when it reaches the subroutine that prints it, and both
// are reached exactly once, at the end.
func drive(m *vm.VM, h *host) error {
	lohalt, hasLohalt := m.Symbols["LOHALT"]
	prenv, hasPrenv := m.Symbols["PRENV"]

	m.PC = m.Registers.Start
	for cycles := 0; cycles < runCycleLimit; cycles++ {
		switch {
		case hasLohalt && m.PC == lohalt:
			h.finalise()
		case hasPrenv && m.PC == prenv:
			h.phase = phaseConstructions
		}
		err := m.Step(nil, nil)
		if err == nil {
			continue
		}
		if errors.Is(err, vm.ErrQuit) || errors.Is(err, vm.ErrHalted) {
			// MDQUIT and HALT are both orderly ends
			if h.fatal != nil {
				return h.fatal
			}
			return nil
		}
		if h.fatal != nil {
			// the machine stopped because the host refused to go on, and the
			// diagnostic is already on the debugging stream
			return h.fatal
		}
		if errors.Is(err, vm.ErrStackOverflow) {
			return ErrNoStorage
		}
		return err
	}
	return fmt.Errorf("%d cycles: %w", runCycleLimit, ErrAborted)
}

// result reads back what the process did.
//
// The two numbers of the "At end of process" line come from the same places
// the finalisation code takes them from: the line count is S2 less one,
// because S2 counts the line being read, and the call count is a variable of
// the MI-logic's own.
func result(m *vm.VM, h *host) Result {
	res := Result{Errors: h.sv(svErrorCount), Lines: h.sv(svLineCount) - 1}
	if address, ok := m.Symbols["INVOCT"]; ok {
		res.Calls = m.Core[address].Value
	}
	res.Fatal = h.fatal != nil
	return res
}

// assemble turns the LOWL source into a machine, in memory and in silence.
//
// The assembler's listings and its commentary are for lasm; a library call
// must not write files into whatever directory it was run from, nor print to
// a stream it does not own.
func assemble(source []byte) (*vm.VM, error) {
	nodes := cst.ParseBuffer(source)
	for _, node := range nodes {
		if node.Error != nil {
			return nil, fmt.Errorf("%d:%d: %w", node.Line, node.Col, node.Error)
		}
	}
	tree, err := ast.Parse(nodes)
	if err != nil {
		return nil, err
	}
	return assembler.Assemble(tree, assembler.Options{})
}

// readSource loads the LOWL source, from the path the job names or from the
// places it is usually unpacked into.
func readSource(name, engine string) ([]byte, error) {
	// A path is the more specific instruction of the two, and it is the one a
	// user reaches for to run something this binary was not built with, so it
	// is answered first.
	if name != "" {
		source, err := os.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, ErrNoEngineSource)
		}
		return source, nil
	}
	if engine != "" {
		return engineSource(engine)
	}
	// Neither was given, so the file system is searched exactly as it was
	// before anything could be embedded. The embedded engines are deliberately
	// *not* consulted here: cmd/ml1 leaves both fields empty and has to keep
	// behaving the way its operating instructions say it does, whatever the
	// binary happens to have been built with. Choosing an engine is maclo's
	// job, and it does it by filling Engine in.
	tried := EnginePaths()
	for _, candidate := range tried {
		if source, err := os.ReadFile(candidate); err == nil {
			return source, nil
		}
	}
	// every place that was looked, because "cannot read the LOWL source" with
	// one path attached reads as though that path were the only answer, and
	// the whole point of the search is that there are several
	return nil, fmt.Errorf("looked in %s: %w", strings.Join(tried, ", "), ErrNoEngineSource)
}
