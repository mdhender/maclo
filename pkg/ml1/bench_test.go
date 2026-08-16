// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ml1

import (
	"io"
	"testing"

	"github.com/mdhender/maclo/pkg/lowl/assembler"
	"github.com/mdhender/maclo/pkg/lowl/ast"
	"github.com/mdhender/maclo/pkg/lowl/cst"
)

// What it costs to build the processor before using it.
//
// This port assembles the whole LOWL source on every run and throws the result
// away when the process ends — there is no cached object file and no
// serialised machine. That is the design, and these benchmarks are what make
// it a measured decision rather than an assumed one.
//
// Measured on an Apple M4, darwin/arm64, against the AJB source (4,400 lines):
//
//	BenchmarkStageCST          2.74 ms    15.8 MB/op
//	BenchmarkStageAST          0.34 ms     0.8 MB/op
//	BenchmarkStageAssemble     1.40 ms     6.1 MB/op
//	BenchmarkAssembleWhole     4.47 ms    22.6 MB/op
//	BenchmarkRunEmpty          4.57 ms    22.7 MB/op
//
// Two things to read off that, both of which are why the numbers are recorded
// here rather than left to be rediscovered:
//
//   - Startup is not a cost. The gap between assembling and running a process
//     that does nothing is about 100µs: laying out the S-variables, sizing the
//     workspace, and driving the machine to LOHALT. Assembly is the whole of
//     it, and the scanner is the majority of assembly.
//
//   - The allocation is the number to watch, not the time. 22MB per assembly,
//     two thirds of it in the cst stage, is fine for a command that assembles
//     once and exits and would not be fine for a process assembling
//     repeatedly. If that ever changes, this is where it shows up first.
//
// For scale: the whole of `maclo` on empty input is ~8.4ms of wall clock,
// against ~6.1ms for the C reference implementation and ~1.2ms for
// /usr/bin/true. Compiling a macro processor before every use costs about two
// milliseconds more than not doing so.
//
// Run them with:
//
//	go test ./pkg/ml1 -run '^$' -bench Benchmark -benchmem

// benchSource is the engine these benchmarks measure. A build with none is a
// legitimate build, so they skip rather than fail.
func benchSource(b *testing.B) []byte {
	b.Helper()
	src, err := engineSource(DefaultEngine())
	if err != nil {
		b.Skipf("no engine is embedded in this build: %v", err)
	}
	return src
}

// BenchmarkStageCST measures bytes to one node per source line. It is the
// most expensive of the four and by far the most allocation.
func BenchmarkStageCST(b *testing.B) {
	src := benchSource(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cst.ParseBuffer(src)
	}
}

// BenchmarkStageAST measures nodes to typed opcodes and parameters.
func BenchmarkStageAST(b *testing.B) {
	src := benchSource(b)
	nodes := cst.ParseBuffer(src)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ast.Parse(nodes); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStageAssemble measures the single pass with back-fill that
// populates Core.
func BenchmarkStageAssemble(b *testing.B) {
	src := benchSource(b)
	nodes := cst.ParseBuffer(src)
	tree, err := ast.Parse(nodes)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := assembler.Assemble(tree, assembler.Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAssembleWhole measures all four stages: source bytes to a machine
// ready to run. This is what every invocation pays before it reads a
// character of the user's input.
func BenchmarkAssembleWhole(b *testing.B) {
	src := benchSource(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := assemble(src); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRunEmpty measures the whole of Run on input that asks the processor
// to do nothing. Subtracting BenchmarkAssembleWhole leaves startup: the
// storage layout and a process that reaches the end immediately.
func BenchmarkRunEmpty(b *testing.B) {
	benchSource(b)
	job := Job{
		Inputs:  []Input{StringInput("bench.ml1", "")},
		Outputs: []io.Writer{io.Discard},
		Debug:   io.Discard,
		Engine:  DefaultEngine(),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Run(job); err != nil {
			b.Fatal(err)
		}
	}
}
