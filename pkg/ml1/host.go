// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/maloquacious/ml_i/pkg/lowl/vm"
)

// host is the MD-logic: everything the LOWL engine cannot know for itself.
//
// It is the vm.Host interface plus the S-variables that go with it. The three
// subroutines it supplies are small, but almost all of the behaviour the
// operating instructions describe lives in them — which stream input comes
// from, which streams output goes to, what the debugging stream will accept —
// so a change to any of that belongs here and not in pkg/lowl.
type host struct {
	job Job
	m   *vm.VM

	svarpt  int // address of the word holding the count of S-variables
	streams []*stream

	// atLineStart says the next character written will be the first of an
	// output line. It belongs to the process rather than to any one output
	// file, because S19 and the listing both count lines that the S21 mask
	// sends nowhere. S24 is the per file version of the same question.
	atLineStart bool

	// phase says which part of the end of process report is being written,
	// and is what S18 is applied to. See finalisation in lowl.go.
	phase phase

	// debugColumn is how far along the current line of the debugging stream
	// we are, for the hard wrap at Job.DebugWidth.
	debugColumn int

	// fatal is the condition that ended the process early, if one did. It is
	// set before the diagnostic is written rather than after, because writing
	// the diagnostic can provoke the same condition again — the quota is
	// charged for it, and the stream it goes on can be the one that failed —
	// and a second report of a process that is already over helps nobody.
	fatal error
}

type phase int

const (
	phaseRunning       phase = iota // the process itself
	phaseStatistics                 // the "At end of process" line
	phaseConstructions              // the list of defined constructions
)

// sv returns the value of Sn, and setSV assigns it. The block is stored in
// reverse with a count above it, which vm.SetSystemVariables laid out and
// SVARPT points at.
func (h *host) sv(n int) int {
	return h.m.Core[h.svarpt-n*h.m.Registers.LNM].Value
}

func (h *host) setSV(n, value int) {
	h.m.Core[h.svarpt-n*h.m.Registers.LNM].Value = value
}

// abortedMessage is the last line of a fatal diagnostic. It is not in the LOWL
// source, because none of the conditions that provoke it is something the
// MI-logic can detect: they are all the host's, and so is this.
const abortedMessage = "Process aborted due to above error\n"

// abort ends the process the way the MI-logic ends one of its own.
//
// The messages appendix AA names are all reported through here, and a
// diagnostic in AA's shape has four parts: the "Error(s)" prologue, which §6.3
// says is what S5 counts; the message; a context print-out saying where the
// process had reached; and a line saying that it stopped there.
//
// Only the prologue and the message can be written from here. The context
// print-out walks ML/I's own stack of environments, so the program is the only
// thing that can produce it — which is why this does what the storage overflow
// handler does and hands control to ER7A, the point in the source that prints
// the context and then goes to the finalisation code. Control does not come
// back, and the last line is written when drive sees the finalisation reached.
//
// message is the wording appendix AA gives, and it is all that reaches the
// debugging stream. err is what the caller of Run gets back: it keeps the
// detail a program needs and a reader of the debugging stream does not, which
// is the sentinel to test with errors.Is and any underlying I/O error.
//
// The error returned is what the host method should return, and it is nil when
// the machine was redirected. The process is over either way; a nil says it
// ends by running ML/I's own error path rather than by the machine stopping.
func (h *host) abort(message string, err error) error {
	if h.fatal != nil {
		return h.fatal
	}
	h.fatal = err
	h.writeError(message)

	er7a, ok := h.m.Symbols["ER7A"]
	if !ok || h.phase != phaseRunning {
		// Two conditions with nowhere to send the machine: a program with no
		// error print-out, the way one with no ERLSO has nowhere to send a
		// stack overflow, and a process already in its finalisation, where the
		// error path leads back to the code that is running. Both say the rest
		// here and stop.
		h.writeMessageText("\n\n" + abortedMessage)
		return h.fatal
	}
	h.m.PC = er7a
	return nil
}

// stop is abort for a condition that cannot let the machine run on.
//
// The quota is the one that matters: printing the context would need more
// lines of the debugging stream than there are, which is the condition itself.
// So the diagnostic is the prologue, the message and the last line, with
// nothing in between — which is what the reference implementation writes.
func (h *host) stop(message string, err error) error {
	if h.fatal != nil {
		return h.fatal
	}
	h.fatal = err
	h.writeError(message)
	h.writeMessageText("\n\n" + abortedMessage)
	return h.fatal
}

// writeError does what PRERR does, for a diagnostic the program is not the one
// reporting: the prologue, and the count in S5 that §6.3 says it is.
//
// The blank line between the prologue and the message is the host's own. The
// MI-logic writes its message straight after "Error(s)" and this does not,
// which is the difference the reference implementation shows between a message
// of ML/I's and a message about the machine it is running on.
func (h *host) writeError(message string) {
	h.setSV(svErrorCount, h.sv(svErrorCount)+1)
	h.writeMessageText("\n\n\nError(s)\n\n" + message)
}

// finalise is called when control reaches the finalisation code, which is
// where a process ends whether it ended well or not. A process that was
// aborted says so here, after the context print-out that ER7A produced and
// before the end of process report that S18 controls.
func (h *host) finalise() {
	if h.phase != phaseRunning {
		return
	}
	if h.fatal != nil {
		h.writeMessageText(abortedMessage)
	}
	h.phase = phaseStatistics
}

// ReadChar is MDREAD.
//
// The order of the tests is the one appendix AA lays down, because a macro can
// set S10 between any two characters and the answers differ: a zero is the end
// of the whole process, a value in 101..105 is a rewind, and reaching the end
// of a stream that is not the revert stream is not the end of anything.
func (h *host) ReadChar() (int, error) {
	for {
		n := h.sv(svInputStream)

		// zero means the user has declared the input finished
		if n == 0 {
			return 0, io.EOF
		}

		// 101..105 select a stream and reposition it at its start. The
		// modified value is stored back, so the rewind happens once.
		if n >= 101 && n <= 100+MaxInputs {
			n -= 100
			h.setSV(svInputStream, n)
			if !h.validStream(n) {
				return 0, h.abort(illegalStream(n))
			}
			if err := h.streams[n-1].rewind(); err != nil {
				// AA.4.1.3. The message says nothing about which stream or why,
				// so the error the caller gets keeps both.
				return 0, h.abort("Cannot rewind input stream",
					fmt.Errorf("input stream %d: %w: %v", n, ErrCannotRewind, err))
			}
		}

		if !h.validStream(n) {
			return 0, h.abort(illegalStream(n))
		}

		ch, err := h.streams[n-1].read()
		if errors.Is(err, io.EOF) {
			// end of file. The process ends only when it is the revert stream
			// that has run out; otherwise input carries on there.
			if n == h.sv(svRevertStream) {
				return 0, io.EOF
			}
			h.setSV(svInputStream, h.sv(svRevertStream))
			continue
		} else if err != nil {
			// AA.4.1 has no message for this one — it names a write error and
			// not a read error — so the wording follows AA.4.1.4's.
			return 0, h.abort(fmt.Sprintf("Error while reading input file %d", n),
				fmt.Errorf("error while reading input file %d: %w", n, err))
		}

		// one character may be translated into another on the way in, which is
		// how a character the user cannot type is fed to a macro. S16 holds
		// -1 when nothing is being translated, and no character has that code.
		if from := h.sv(svTranslate); from >= 0 && ch == from {
			ch = h.sv(svTranslateTo)
		}
		return ch, nil
	}
}

// validStream reports whether n names an input stream the user gave.
func (h *host) validStream(n int) bool {
	return 1 <= n && n <= len(h.streams)
}

// illegalStream is AA.4.1.2, both ways round. The quotes around the value are
// not AA's — it writes "viz n" — but they are what the reference
// implementation writes, and they are how the other "has illegal value"
// messages of the MI-logic write theirs.
func illegalStream(n int) (string, error) {
	message := fmt.Sprintf(`S10 has illegal value, viz "%d"`, n)
	return message, fmt.Errorf("%s: %w", message, ErrIllegalStream)
}

// WriteChar is MDOUCH.
//
// S21 is a bit per output stream rather than a stream number, so one character
// may go to several files at once or to none at all, and a character aimed at
// a file the user never asked for is silently dropped rather than being an
// error.
//
// The line count and the listing are not part of that. Both are taken from the
// character as it arrives here, before the mask is applied, so a line that the
// user has masked away still counts and still appears in the listing.
func (h *host) WriteChar(ch int) error {
	if h.atLineStart {
		// S19 is the number of the line being written, and it is stepped on
		// the first character of a line rather than on the newline that ends
		// the one before. The difference is visible: it is what a macro reads
		// between two lines, and what the listing prints against this one.
		h.setSV(svOutputLine, h.sv(svOutputLine)+1)
	}
	if err := h.list(ch); err != nil {
		return err
	}

	mask, deprecated := h.sv(svOutputMask), h.sv(svOutputTwo)
	for i, w := range h.job.Outputs {
		// a nonzero S22 also writes to the second file, and does not write to
		// it twice when S21 has asked for it as well.
		if mask&(1<<i) == 0 && !(i == 1 && deprecated != 0) {
			continue
		}
		if _, err := w.Write([]byte{byte(ch)}); err != nil {
			// AA.4.1.4, whose message names the file the error was on
			return h.abort(fmt.Sprintf("Error while writing to output file %d", i+1),
				fmt.Errorf("error while writing to output file %d: %w", i+1, err))
		}
		h.setAtLineStart(i, ch == '\n')
	}
	h.atLineStart = ch == '\n'
	return nil
}

// list copies one character of the output text to the listing.
//
// S20 chooses between three things: zero produces no listing at all, two
// produces one with the number of each line in front of it, and any other
// value produces one without.
//
// It is read for every character rather than once a line, so a macro can turn
// the listing on and off around the part of the output it cares about. Nothing
// marks where the missing text was, and the line numbers carry on from wherever
// the count has reached rather than from one, so a listing that was interrupted
// says so by the numbers it skips.
//
// A nil Listing means the user did not ask for one, whatever S20 says.
func (h *host) list(ch int) error {
	control := h.sv(svListing)
	if h.job.Listing == nil || control == 0 {
		return nil
	}
	if h.atLineStart && control == listingNumbered {
		if err := h.writeListing(fmt.Appendf(nil, "%5d.   ", h.sv(svOutputLine))); err != nil {
			return err
		}
	}
	// one byte, not a rune: the character set is the 8 bit one appendix AA
	// names, and encoding it would turn a code above 127 into two characters.
	return h.writeListing([]byte{byte(ch)})
}

func (h *host) writeListing(text []byte) error {
	if _, err := h.job.Listing.Write(text); err != nil {
		return h.abort("Error while writing to listing file",
			fmt.Errorf("error while writing to listing file: %w", err))
	}
	return nil
}

// setAtLineStart records in S24 whether output stream i is at the start of a
// line, so that a macro can avoid generating a blank one. Streams the user did
// not ask for are at the start of a line at all times, and this never clears
// their bits because nothing is ever written to them.
func (h *host) setAtLineStart(i int, atStart bool) {
	flags := h.sv(svAtLineStart)
	if atStart {
		flags |= 1 << i
	} else {
		flags &^= 1 << i
	}
	h.setSV(svAtLineStart, flags)
}

// WriteMessage is MDERCH, and MESS arrives here too.
//
// Two things happen on this stream that do not happen on the results stream.
// It is metered — S12 holds a quota of lines and the process is abandoned when
// it runs out — and it is wrapped, because the golden files of the upstream
// suite were recorded from a device 72 columns wide.
func (h *host) WriteMessage(ch int) error {
	if !h.wantMessage() {
		return nil
	}
	if h.job.DebugWidth > 0 && ch != '\n' && h.debugColumn >= h.job.DebugWidth {
		if err := h.writeDebug('\n'); err != nil {
			return err
		}
	}
	if err := h.writeDebug(ch); err != nil {
		return err
	}
	if ch != '\n' {
		return nil
	}

	// the end of process report is not charged against the quota
	if h.phase != phaseRunning {
		return nil
	}
	// nor is the diagnostic of a process that is already dying: what remains
	// to be written is the context print-out that says why, and there is no
	// sense in a quota cutting that off.
	if h.fatal != nil {
		return nil
	}
	h.setSV(svDebugQuota, h.sv(svDebugQuota)-1)
	if h.sv(svDebugQuota) < 0 {
		return h.stop("Debugging file lines quota exhausted", ErrDebugQuota) // AA.4.1.1
	}
	return nil
}

// wantMessage applies S18 to the end of process report. Both of its bits are
// clear to begin with, so a process that finishes cleanly writes nothing here.
func (h *host) wantMessage() bool {
	switch h.phase {
	case phaseStatistics:
		return h.sv(svReport)&reportStatistics != 0
	case phaseConstructions:
		return h.sv(svReport)&reportConstructions != 0
	}
	return true
}

func (h *host) writeDebug(ch int) error {
	if _, err := h.job.Debug.Write([]byte{byte(ch)}); err != nil {
		// the one fatal condition with no diagnostic, because the stream a
		// diagnostic would go on is the stream that failed
		if h.fatal == nil {
			h.fatal = fmt.Errorf("error while writing to debugging file: %w", err)
		}
		return h.fatal
	}
	if ch == '\n' {
		h.debugColumn = 0
	} else {
		h.debugColumn++
	}
	return nil
}

// writeMessageText puts a diagnostic of the host's own on the debugging
// stream. It goes through the same wrap as everything else, but not through
// the quota: the process is over by the time this is called.
func (h *host) writeMessageText(text string) {
	saved := h.phase
	h.phase = phaseRunning
	for i := 0; i < len(text); i++ {
		if h.job.DebugWidth > 0 && text[i] != '\n' && h.debugColumn >= h.job.DebugWidth {
			_ = h.writeDebug('\n')
		}
		_ = h.writeDebug(int(text[i]))
	}
	h.phase = saved
}

// stream is one input stream, held open across a switch to another one so
// that coming back to it carries on where it left off.
type stream struct {
	in     Input
	closer io.ReadCloser
	reader *bufio.Reader
	eof    bool
}

func (s *stream) open() error {
	rc, err := s.in.Open()
	if err != nil {
		return err
	}
	s.closer, s.reader, s.eof = rc, bufio.NewReader(rc), false
	return nil
}

func (s *stream) rewind() error {
	s.close()
	return s.open()
}

func (s *stream) close() {
	if s.closer != nil {
		_ = s.closer.Close()
	}
	s.closer, s.reader = nil, nil
}

// read returns the next character, converting whatever the file uses to end a
// line into a single newline.
func (s *stream) read() (int, error) {
	if s.eof {
		return 0, io.EOF
	}
	if s.reader == nil {
		if err := s.open(); err != nil {
			return 0, err
		}
	}
	b, err := s.reader.ReadByte()
	if errors.Is(err, io.EOF) {
		s.eof = true
		return 0, io.EOF
	} else if err != nil {
		return 0, err
	}
	if b == '\r' {
		// a carriage return is either half of a pair or a line ending on its
		// own; either way one newline reaches the logic.
		if next, err := s.reader.ReadByte(); err == nil && next != '\n' {
			_ = s.reader.UnreadByte()
		}
		b = '\n'
	}
	return int(b), nil
}
