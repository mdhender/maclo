# How the executable works

A high-level tour of what `ml1` actually does when you run it. For the detail — source versions, the
LOWL extensions, the memory layout — see
[running ML/I on the LOWL VM](docs/explanation/running-ml1-on-the-lowl-vm.md).

## The surprising part first

There is no ML/I in the `ml1` binary. There is a *computer* in it.

ML/I was published as source in **LOWL** — a low-level, machine-independent language P.J. Brown
designed so that one program could be moved to any machine by writing a small assembler for it. The
usual way to port ML/I is to hand-translate that LOWL into your language. This doesn't. It
implements the LOWL machine and runs Brown's 1986 source unmodified, as data, loaded at startup.

So `ml1` is an emulator that happens to boot a macro processor.

## What happens when you run `ml1 hello.ml1`

**1. Find the engine.** `pkg/ml1/engine.go` walks the search order (`-s`, `$ML1_LOWL_SOURCE`, the
per-user directory, then `.downloads/` in a checkout) until it finds `ml1ajb.lwl` — 57KB of LOWL,
about 4,400 lines. Nothing can happen until it does, which is why `--fetch-engine` exists.

**2. Compile it, in memory, every time.** Four stages, wired in `assemble` at `pkg/ml1/lowl.go:149`:

```
ml1ajb.lwl
  → scanner    bytes → tokens
  → cst        tokens → one node per source line, errors carried on the node
  → ast        nodes → Op + typed parameters
  → assembler  ast → a VM with Core[] populated
```

This takes a few milliseconds and is thrown away when the process exits — there is no cached object
file. The assembler is single-pass with back-fill: forward references queue on a symbol table and
get patched at the end.

**3. Lay out memory.** `pkg/ml1/lowl.go:63` writes the S-variables into low core first — ML/I
*reads* those and never builds them, so the host has to — and puts the workspace behind them.
Address 0 is a `HALT`, so a wild jump stops rather than wanders.

**4. Run it.** `drive` at `pkg/ml1/lowl.go:93` sets the PC to the `BEGIN` label and steps the
machine until it halts, capped at 500 million cycles as a runaway guard.

## The machine

`vm.VM` is a handful of registers plus `Core [65536]Word`. The twist is that a `Word` isn't 16 bits
of anything — it's a struct carrying an opcode, two values, a text field, and the source line it came
from. Code and data share the array, so a variable is just a `CON` word whose `Value` field gets read
and written, and core doubles as a program listing you can disassemble.

Multi-exit subroutines — a LOWL feature with no direct machine equivalent — assemble into a
contiguous jump table.

## The host boundary

This is where the design pays off. LOWL deliberately leaves a short list of operations to whoever
ports it, because they're the only things that differ between machines. In this port that list is an
interface (`pkg/lowl/vm/host.go:21`) with essentially three methods:

```go
ReadChar() (int, error)    // next character of source text
WriteChar(ch int) error    // one character to the results stream
WriteMessage(ch int) error // one character to the debugging stream
```

**Everything ML/I-specific that isn't in the LOWL source lives behind those three calls.** Input
stream switching and rewinding, `S10`, newline conversion, the output options, the listing and its
line numbers, the debugging quota — none of that is in Brown's source, so `pkg/ml1/host.go` supplies
it. ML/I asks for one character at a time and never learns there are files.

That's also why `ml1.Run` can work entirely from buffers: the host is handed `io.Writer`s, and the
library writes no files and prints to no stream it wasn't given.

## How it ends

`drive` isn't just `vm.Run` — it watches for two addresses. When the PC reaches `LOHALT` or `PRENV`,
the end-of-process report is starting, and the host needs to know which part, because `S18` (which
controls that report) postdates the 1986 source and the source writes the report unconditionally.
Watching where control *goes* is the only way to honour a variable the program has never heard of.

The same trick handles fatal errors in reverse. When the host refuses to continue — an illegal `S10`,
a write failure — it doesn't stop the machine. It writes what it can and then *jumps the program to
`ER7A`*, ML/I's own error path, so the diagnostic comes out with ML/I's real context print-out
walking back through the macro stack. A Go error string would have been one flat line.

## Why bother

The payoff is fidelity. Bugs in a hand translation are invisible — they look like code. Here, the
logic is the original artifact, so a mismatch against the reference implementation is either a bug in
the ~60 opcodes or in the host, both small enough to test exhaustively. That's how the two nastiest
bugs got found: `RL` storing an address instead of a distance, and `CSS` popping one stack link
instead of clearing the stack. Both survived a clean assembly *and* a passing conformance suite, and
neither is the kind of thing you'd spot reading translated Go.

The cost is the one you already met: the engine is a runtime dependency that can't legally ship in
the binary.
