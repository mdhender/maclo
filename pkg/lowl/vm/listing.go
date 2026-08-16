// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package vm

import (
	"bytes"
	"fmt"
	"os"
)

func (m *VM) Disassemble(name string) error {
	b := &bytes.Buffer{}
	for pc, word := range m.Core[:m.Registers.Last] {
		if word.Source.Continuation {
			continue
		}
		_, _ = fmt.Fprintf(b, "%4d %-8s %6d %6d ;; %4d %-8s %s\n", pc, word.Op, word.Value, word.ValueTwo, word.Source.Line, word.Source.Op, word.Source.Parameters)
	}
	return os.WriteFile(name, b.Bytes(), 0644)
}
