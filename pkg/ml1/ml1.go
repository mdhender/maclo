// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

// Package ml1 implements the ML/I macro processor.
//
// The command in cmd/ml1 is a thin wrapper over Run, and the golden file
// tests drive Run directly, in process, from buffers. Nothing here opens or
// closes a file on the caller's behalf; a Job is built from readers and
// writers the caller owns.
//
// The model comes from appendix AA of the user's manual, which is worth
// reading before changing any of this. Two parts of it are easy to get wrong:
//
//   - Input files are numbered streams, not one concatenated source. The
//     system variable S10 selects the current stream, setting it to a value
//     in 101..105 rewinds a stream, and at end of file the processor reverts
//     to the stream named by S23. The process ends only when the revert
//     stream itself runs out. This is why Job.Inputs is a list of re-openable
//     sources rather than an io.MultiReader.
//
//   - Output files are selected by a bit mask in S21, so one piece of text
//     may go to several output streams at once, or to none at all. Writes
//     aimed at a stream that was not given on the command line are discarded.
package ml1

import (
	"fmt"
	"io"
)

const (
	// MaxInputs and MaxOutputs are the limits imposed by the operating
	// instructions: at most five input streams and four output streams.
	MaxInputs  = 5
	MaxOutputs = 4

	// DefaultWorkspace is the amount of workspace, in words, given to a
	// process when the caller does not ask for a size.
	DefaultWorkspace = 5000

	// DefaultDebugWidth is the column at which the reference implementation
	// hard wraps the debugging stream. Its golden files are wrapped there, so
	// matching them means wrapping in the same place.
	DefaultDebugWidth = 72

	// NeverWrap is the Job.DebugWidth that emits every line whole. It is the
	// zero value on purpose: a Job built by hand does not transform what the
	// engine writes unless it is asked to.
	NeverWrap = 0

	// DebugInsertLimit is 2N from chapter 6 of the user's manual, the longest
	// run of the user's own text copied into an error message before it is
	// truncated.
	DebugInsertLimit = 64

	// DefaultDebugQuota is the initial value of S12, the number of lines the
	// debugging stream will still accept. The process aborts when it goes
	// negative.
	DefaultDebugQuota = 500
)

// Job describes one run of the processor.
//
// The zero value is not usable: at least one input stream, one output stream,
// and a debugging stream are required. Run never closes anything in Outputs,
// Debug, or Listing; it does open and close each Input.
type Job struct {
	// Inputs are the input streams in the order they were named on the
	// command line. Inputs[0] is stream 1, where processing starts.
	Inputs []Input

	// Outputs are the output streams in the order they were named on the
	// command line. Outputs[0] is output stream 1. S21 is a bit mask over
	// these, so text may reach several at once.
	Outputs []io.Writer

	// Debug receives error messages, MCNOTE output, and the end of process
	// report. It is the file named by -d, and the standard error otherwise.
	Debug io.Writer

	// Listing receives the listing named by -l, under the control of S20.
	// A nil Listing means no listing was asked for, whatever S20 says.
	Listing io.Writer

	// Workspace is the size of the work area in words. Zero means
	// DefaultWorkspace.
	Workspace int

	// LOWLSource is the path to the LOWL source of ML/I that the engine
	// assembles and runs.
	//
	// It is a path rather than something built in because the source's licence
	// forbids redistributing a machine readable copy, so it cannot live in
	// this repository; pointing this at a newer version is also all it takes
	// to run one. An empty string looks in the places the archives are
	// usually unpacked into, relative to the working directory, and Run
	// reports ErrNoEngineSource if it is not there.
	LOWLSource string

	// Engine names a LOWL source built into this binary, as Engines() lists
	// them — "ml1ajb" rather than a path. An empty string means none was
	// chosen.
	//
	// The two are separate fields because they are separate questions, and a
	// caller may answer either, neither, or both. LOWLSource wins if both are
	// set: a path is the more specific instruction, and it is the one a user
	// reaches for to run something the binary was not built with. With
	// neither, Run searches the file system exactly as it did before engines
	// could be embedded, which is what keeps cmd/ml1's behaviour unchanged.
	Engine string

	// DebugWidth is the column at which lines written to Debug are hard
	// wrapped, mid word, the way the reference implementation does it.
	// NeverWrap, the zero value, emits each line whole.
	//
	// This is a knob rather than a constant because the two golden corpora
	// want different answers: the suite from ml1.org.uk was produced at
	// DefaultDebugWidth and has to be matched there, while the corpus in this
	// repository is run at NeverWrap so that its golden files record the
	// message text itself instead of an artifact of a 72 column device.
	DebugWidth int
}

// Result reports what a process did.
//
// It is meaningful even when Run returns ErrAborted or ErrProcessErrors,
// because the reference implementation writes its diagnostics before it gives
// up. Errors is the final value of S5; Lines and Calls are the two numbers in
// the "At end of process" line that S18 asks for.
type Result struct {
	Errors int
	Lines  int
	Calls  int
	Fatal  bool
}

// ExitStatus maps a Result onto the exit status the operating instructions
// ask for: 255 when a fatal error ended the process early, 254 when the
// process finished but reported errors, and 0 otherwise.
func (r Result) ExitStatus() int {
	switch {
	case r.Fatal:
		return 255
	case r.Errors != 0:
		return 254
	}
	return 0
}

// Validate reports whether a job can be run at all.
//
// Run calls it, and it is exported so that a caller can reject a bad command
// line before it truncates any output file.
func (j Job) Validate() error {
	switch {
	case len(j.Inputs) == 0:
		return ErrNoInput
	case len(j.Inputs) > MaxInputs:
		return fmt.Errorf("%d streams: %w (at most %d)", len(j.Inputs), ErrTooManyInputs, MaxInputs)
	case len(j.Outputs) == 0:
		return ErrNoOutput
	case len(j.Outputs) > MaxOutputs:
		return fmt.Errorf("%d streams: %w (at most %d)", len(j.Outputs), ErrTooManyOutputs, MaxOutputs)
	case j.Debug == nil:
		return ErrNoDebug
	case j.Workspace < 0:
		return fmt.Errorf("%d words: %w", j.Workspace, ErrWorkspace)
	case j.DebugWidth < 0:
		return fmt.Errorf("%d columns: %w", j.DebugWidth, ErrDebugWidth)
	}
	for i, in := range j.Inputs {
		if in.Open == nil {
			return fmt.Errorf("input stream %d (%s): %w", i+1, in.Name, ErrNoInput)
		}
	}
	for i, w := range j.Outputs {
		if w == nil {
			return fmt.Errorf("output stream %d: %w", i+1, ErrNoOutput)
		}
	}
	return nil
}

// Run performs the process described by job.
//
// It returns a Result even when it returns an error, so that a caller can
// still report an exit status for a process that was aborted.
//
// Validation happens before anything else, so a caller that builds a bad Job
// gets a real error rather than one from deep inside the engine.
//
// This is the switch the package was shaped around. There is one backend
// today, the LOWL one, which assembles the distributed LOWL source of ML/I and
// runs it; an L backend would be a second case here and nothing else in the
// package would change.
func Run(job Job) (Result, error) {
	if err := job.Validate(); err != nil {
		return Result{Fatal: true}, err
	}
	if job.Workspace == 0 {
		job.Workspace = DefaultWorkspace
	}
	return runLOWL(job)
}
