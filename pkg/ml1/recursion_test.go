// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package ml1_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/ml1"
)

// nestSource is a macro that recurses to a given depth, writing a bracket on
// the way down and the matching one on the way back up. The brackets are the
// point: text emitted *after* the recursive call cannot be written until the
// call returns, so a correct result is only possible if the whole stack was
// held and then unwound. A countdown that printed on the way down would pass
// with the unwinding broken.
func nestSource(depth int) string {
	return fmt.Sprintf(`MCSKIP MT,<>
MCINS %%.
MCDEF NEST; AS <MCGO L1 IF %%A1. GR 0
.<>MCGO L0
%%L1.[NEST %%%%A1.-1.;]>
NEST %d;
`, depth)
}

// nestWant is what nestSource(depth) produces: depth open brackets, the dot
// the base case writes, depth closing brackets, and the newline that follows
// the call in the source.
func nestWant(depth int) string {
	return strings.Repeat("[", depth) + "." + strings.Repeat("]", depth) + "\n"
}

// TestDeepRecursion covers recursion deep enough for the workspace to matter.
//
// Nothing in the local corpus is workspace sensitive: its deepest case recurses
// a handful of levels and would produce the same output with any workspace at
// all. That leaves a gap on both sides. Nothing checks that a deep recursion
// unwinds correctly, and nothing checks that the workspace is what limits it,
// so a regression that quietly shrank the effective workspace would show up
// only as an upstream case that was already failing for other reasons.
//
// This is not a golden file, deliberately. The depth at which storage runs out
// is implementation dependent -- it is why the upstream overflow case has its
// debugging stream excluded -- so what is pinned here is the shape of the
// dependence and never the boundary. The two workspaces below are far apart
// for that reason: the reference implementation and this one both fail at
// 12000 words and both succeed at 20000, but that agreement is not something a
// test should rely on.
//
// Every case was run against the oracle first. At the default workspace it
// completes depth 50 and abandons depth 200; given 24000 words it completes
// depth 200 and writes the same 402 bytes this engine does.
func TestDeepRecursion(t *testing.T) {
	requireEngine(t)

	t.Run("moderate depth needs no extra workspace", func(t *testing.T) {
		const depth = 50
		out, dbg, res, err := runNest(t, depth, ml1.DefaultWorkspace)
		if err != nil {
			t.Fatalf("run: %v\ndebugging stream:\n%s", err, dbg)
		}
		if got, want := out, nestWant(depth); got != want {
			t.Errorf("output: want %d bytes, got %d:\n%s", len(want), len(got), got)
		}
		if dbg != "" {
			t.Errorf("debugging stream: want nothing, got:\n%s", dbg)
		}
		if res.Errors != 0 {
			t.Errorf("S5: want 0 errors, got %d", res.Errors)
		}
	})

	// The same depth twice, with and without the room for it. Only the
	// workspace differs, so it is the workspace that decides.
	const deep = 200

	t.Run("deep with room to hold the stack", func(t *testing.T) {
		out, dbg, res, err := runNest(t, deep, 24000)
		if err != nil {
			t.Fatalf("run: %v\ndebugging stream:\n%s", err, dbg)
		}
		if got, want := out, nestWant(deep); got != want {
			t.Errorf("output: want %d bytes, got %d", len(want), len(got))
			// the brackets are unreadable in bulk, so report where they part
			for i := 0; i < len(got) && i < len(want); i++ {
				if got[i] != want[i] {
					t.Errorf("first difference at byte %d: want %q, got %q", i, want[i], got[i])
					break
				}
			}
		}
		if dbg != "" {
			t.Errorf("debugging stream: want nothing, got:\n%s", dbg)
		}
		if got, want := res.ExitStatus(), 0; got != want {
			t.Errorf("exit status: want %d, got %d", want, got)
		}
	})

	t.Run("deep without it", func(t *testing.T) {
		out, dbg, _, err := runNest(t, deep, ml1.DefaultWorkspace)
		// Which error, and how far it got, are both implementation dependent;
		// that it did not finish is not.
		if err == nil {
			t.Fatalf("run: want the process to give up, got a clean finish")
		}
		if out == nestWant(deep) {
			t.Errorf("output: want an incomplete one, got the whole thing")
		}
		if dbg == "" {
			t.Errorf("debugging stream: want a diagnostic, got nothing")
		}
	})
}

// runNest runs nestSource at a given depth and workspace. It skips rather than
// fails when the engine is absent, the same as every other test here.
func runNest(t *testing.T, depth, workspace int) (out, dbg string, res ml1.Result, err error) {
	t.Helper()
	var outBuf, dbgBuf bytes.Buffer
	res, err = ml1.Run(ml1.Job{
		Inputs:     []ml1.Input{ml1.StringInput("nest.ml1", nestSource(depth))},
		Outputs:    []io.Writer{&outBuf},
		Debug:      &dbgBuf,
		Workspace:  workspace,
		DebugWidth: ml1.NeverWrap,
		LOWLSource: lowlSource(),
	})
	if errors.Is(err, ml1.ErrNoEngineSource) {
		t.Skipf("%v; run: go run ./cmd/fetchtestdata\n", err)
	}
	return outBuf.String(), dbgBuf.String(), res, err
}
