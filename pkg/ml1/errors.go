// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1

import "errors"

// ErrAborted and ErrProcessErrors report the two ways a process can end
// badly. Neither is a failure of the caller: the reference implementation
// writes its diagnostics to the debugging stream and then gives up, so the
// output and the debugging stream are still worth comparing against a golden
// file. The upstream overflow and errors tests both need this.
var (
	// ErrAborted means a fatal condition ended the process early: no storage
	// left, an illegal input stream, or the debugging line quota running out.
	// It maps to the exit status 255.
	ErrAborted = errors.New("ml1: process aborted")

	// ErrProcessErrors means the process ran to completion but reported at
	// least one error, so S5 was not zero at the end. It maps to the exit
	// status 254.
	ErrProcessErrors = errors.New("ml1: process errors")
)

// The rest are configuration errors. They are reported by Validate before
// anything is read or written, so a caller can reject a bad command line
// without having created any output files.
var (
	ErrNoInput        = errors.New("ml1: no input stream")
	ErrNoOutput       = errors.New("ml1: no output stream")
	ErrNoDebug        = errors.New("ml1: no debugging stream")
	ErrTooManyInputs  = errors.New("ml1: too many input streams")
	ErrTooManyOutputs = errors.New("ml1: too many output streams")
	ErrWorkspace      = errors.New("ml1: workspace must not be negative")
	ErrDebugWidth     = errors.New("ml1: debugging width must not be negative")
)

// ErrCannotRewind is reported by an Input that cannot be read a second time,
// which is what the standard input is like. A macro that sets S10 to a value
// in 101..105 asks for a rewind, so that combination is a process error
// rather than a programming mistake.
var ErrCannotRewind = errors.New("ml1: cannot rewind input stream")

// ErrIllegalStream is reported when S10 names a stream that was not given on
// the command line, ErrDebugQuota when S12 runs out, and ErrNoStorage when the
// workspace is exhausted. All three abort the process.
var (
	ErrIllegalStream = errors.New("ml1: input stream has an illegal value")
	ErrDebugQuota    = errors.New("ml1: debugging file lines quota exhausted")
	ErrNoStorage     = errors.New("ml1: process aborted for lack of storage")
)

// ErrNoEngineSource means the LOWL source of ML/I could not be read.
//
// It is a sentinel of its own because the engine exists and works: what is
// missing is the source it runs, which cannot be committed to this repository
// because its licence forbids redistributing a machine readable copy. The
// golden harness skips on this, so a clone that has not fetched the archives
// says so plainly instead of failing every case.
var ErrNoEngineSource = errors.New("ml1: cannot read the LOWL source of ML/I")
