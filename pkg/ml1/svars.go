// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1

// The system variables, and the ones this implementation gives a meaning to.
//
// S1 to S9 belong to ML/I itself and mean the same thing everywhere. From S10
// up they are the host's, and what they mean is what appendix AA of the user's
// manual says they mean, because that appendix describes this implementation.
//
// The MI-logic reads only S2, S3, S4 and S5, and S6 is read by the machine
// itself, in GOPC. Everything else here is read and written by the host, so a
// variable that is listed but never mentioned again is one the user can set and
// nothing looks at.
const (
	svStartLine    = 1  // insert the imaginary startline character on input
	svLineCount    = 2  // current source line
	svQuietWarning = 3  // suppress the message for a warning marker with no name
	svQuietNote    = 4  // suppress the context print-out after MCNOTE
	svErrorCount   = 5  // count of processing errors
	svPseudoAlpha  = 6  // code of a character to be counted as alphanumeric
	svInputStream  = 10 // selects the input stream; 101..105 rewind
	svDebugQuota   = 12 // lines the debugging stream will still accept
	svTranslate    = 16 // code of a character to translate on input
	svTranslateTo  = 17 // what S16 becomes
	svReport       = 18 // what the end of process report contains
	svOutputLine   = 19 // current output line
	svListing      = 20 // listing control
	svOutputMask   = 21 // bit per output stream, saying whether to write to it
	svOutputTwo    = 22 // deprecated: a nonzero value also writes to stream 2
	svRevertStream = 23 // the stream input reverts to at end of file
	svAtLineStart  = 24 // bit per output stream, saying it is at the start of a line

	// SystemVariables is how many there are. Setting S25 is an error, which
	// is one way to tell that this number is part of the interface rather
	// than an internal limit.
	SystemVariables = 24
)

// The two bits of S18. Neither is set to begin with, so a clean process writes
// nothing at all to the debugging stream.
const (
	reportConstructions = 1 // 2^0: list the constructions that are defined
	reportStatistics    = 2 // 2^1: the "At end of process" line
)

// listingNumbered is the one value of S20 that means something in particular.
// Zero produces no listing and every other value produces one without line
// numbers, so this is the only comparison worth naming.
const listingNumbered = 2

// systemVariables returns the initial S-variable block, S1 first.
//
// The values are the reference implementation's, read out of it directly with
// an input that inserts each variable in turn rather than taken on trust from
// the manual. Only two of them are surprising. S6 and S16 are -1 because they
// hold character codes and -1 is the code no character has, which is how the
// features they control are turned off. S24 is 15 rather than 1 because it
// holds a bit per output stream saying that stream is at the start of a line,
// and at the start of a process every stream is, including the ones the user
// never asked for.
//
// S19 is not listed here, and that is the answer rather than an omission: it
// starts at zero and reaches one when the first character of the first output
// line is written. Inserting it is what fooled the first reading — the
// insertion is itself output, so it steps the count before it can report it.
func systemVariables() []int {
	values := make([]int, SystemVariables)
	values[svPseudoAlpha-1] = -1
	values[svInputStream-1] = 1
	values[svDebugQuota-1] = DefaultDebugQuota
	values[svTranslate-1] = -1
	values[svOutputMask-1] = 1
	values[svRevertStream-1] = 1
	values[svAtLineStart-1] = 1<<MaxOutputs - 1
	return values
}
