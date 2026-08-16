// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm

import "io"

// Host is the outside world, as the LOWL logic sees it.
//
// A LOWL program does no I/O of its own. It calls machine dependent
// subroutines, and the seven ML/I asks for are the entire boundary between
// its logic and its surroundings. Three of them are policy the machine cannot
// know — which stream a character comes from, which streams it goes to — and
// those are the three here. The rest (MDCONV, MDFIND, MDOP) only read and
// write the program's own variables, so the machine implements them itself,
// and MDQUIT is just an orderly stop.
//
// Keeping the interface here rather than taking the host's type keeps pkg/lowl
// free of any dependency on ML/I: the host is whoever supplies the streams.
type Host interface {
	// ReadChar returns the next character of the source text. It returns
	// io.EOF when the source is exhausted, which is MDREAD's first exit; the
	// stream switching, the rewinds and the newline conversion that decide
	// when that happens all belong to the host.
	ReadChar() (int, error)

	// WriteChar writes one character to the results stream, under whatever
	// output options are in force. A character aimed at a stream the user did
	// not ask for is discarded rather than being an error.
	WriteChar(ch int) error

	// WriteMessage writes one character to the messages stream, which the
	// ML/I manual calls the debugging file. MESS goes here too: the LOWL
	// manual defines it as output to the message stream, so an implementation
	// that sent it anywhere else would put the end of process report in with
	// the results.
	WriteMessage(ch int) error
}

// writerHost is the fallback for a machine that was given writers instead of a
// host. It is enough to run a program such as LOWLTEST, whose I/O requirement
// is one message stream and no input at all.
type writerHost struct {
	stdout io.Writer
	stdmsg io.Writer
}

func (h writerHost) ReadChar() (int, error) {
	return 0, ErrNoHost
}

func (h writerHost) WriteChar(ch int) error {
	printf(h.stdout, "%s", string(rune(ch)))
	return nil
}

func (h writerHost) WriteMessage(ch int) error {
	printf(h.stdmsg, "%s", string(rune(ch)))
	return nil
}

// host returns the host to use for one Step. The writers are the ones Step was
// given, so a caller that steps the machine by hand still gets its output.
func (m *VM) host(stdout, stdmsg io.Writer) Host {
	if m.Host != nil {
		return m.Host
	}
	return writerHost{stdout: stdout, stdmsg: stdmsg}
}
