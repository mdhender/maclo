// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// Input is one of the numbered input streams.
//
// Open is called once when Run starts and again every time the stream is
// rewound, which is what setting S10 to a value in 101..105 asks for. A
// stream that cannot be read a second time reports ErrCannotRewind from the
// second call.
//
// Name appears in messages only. Run closes whatever Open returns.
type Input struct {
	Name string
	Open func() (io.ReadCloser, error)
}

// BytesInput returns an Input that reads data and rewinds freely. It is what
// lets the golden file tests run the processor entirely in memory.
func BytesInput(name string, data []byte) Input {
	return Input{
		Name: name,
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
}

// StringInput is BytesInput for a string.
func StringInput(name, text string) Input {
	return BytesInput(name, []byte(text))
}

// FileInput returns an Input that opens name each time it is read, so a
// rewind costs a second open rather than a buffer.
func FileInput(name string) Input {
	return Input{
		Name: name,
		Open: func() (io.ReadCloser, error) {
			f, err := os.Open(name)
			if err != nil {
				return nil, err
			}
			return f, nil
		},
	}
}

// StreamInput returns an Input that reads r exactly once. A second open
// reports ErrCannotRewind, which is how the standard input behaves.
//
// Use BytesInput instead when the text is already in memory and a macro might
// rewind it.
func StreamInput(name string, r io.Reader) Input {
	used := false
	return Input{
		Name: name,
		Open: func() (io.ReadCloser, error) {
			if used {
				return nil, fmt.Errorf("%s: %w", name, ErrCannotRewind)
			}
			used = true
			return io.NopCloser(r), nil
		},
	}
}
