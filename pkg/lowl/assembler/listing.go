// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package assembler

import (
	"bytes"
	"fmt"
	"github.com/maloquacious/ml_i/pkg/lowl/vm"
	"os"
	"sort"
)

func Listing(name string, machine *vm.VM, symtab *symbolTable) error {
	// create a map for labels
	labels := make(map[int][]string)
	for _, sym := range symtab.symbols {
		if sym.kind == "address" && sym.line != 0 {
			labels[sym.address] = append(labels[sym.address], sym.name)
		}
	}
	for pc := range labels {
		vv := labels[pc]
		sort.Strings(vv)
		labels[pc] = vv
	}

	b := &bytes.Buffer{}
	for pc, word := range machine.Core[:machine.Registers.Last] {
		if word.Source.Continuation {
			continue
		}
		printedPC := false
		for _, label := range labels[pc] {
			if printedPC {
				_, _ = fmt.Fprintf(b, "%4s %4s ", "", "")
			} else {
				_, _ = fmt.Fprintf(b, "%4d %4d ", word.Source.Line, pc)
			}
			_, _ = fmt.Fprintf(b, "[%s]\n", label)
			printedPC = true
		}
		if printedPC {
			_, _ = fmt.Fprintf(b, "%4s %4s ", "", "")
		} else {
			_, _ = fmt.Fprintf(b, "%4d %4d ", word.Source.Line, pc)
		}
		_, _ = fmt.Fprintf(b, "%-8s %6d %6d ;; %-8s %s\n", word.Op, word.Value, word.ValueTwo, word.Source.Op, word.Source.Parameters)
	}
	return os.WriteFile(name, b.Bytes(), 0644)
}
