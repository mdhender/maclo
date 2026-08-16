# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go port of ML/I (a general purpose macro processor). ML/I is distributed as source written in
LOWL, a low-level assembly-like language. Rather than translate ML/I by hand, this repo builds the
tooling to run LOWL: a scanner, parser, assembler, and virtual machine. See
<http://www.ml1.org.uk/> for the LOWL spec and the upstream sources.

`pkg/lowl/README.md` contains the LOWL VM description and the full opcode table with argument
conventions — read it before touching opcode handling.

## Commands

```sh
go build ./...
go test ./...
go test ./pkg/lowl/vm -run TestVM -v     # the VM opcode tests (one big table-free test func)
go run ./cmd/lasm --source path/to/file.lowl
go run ./cmd/lasm --source file.lowl --test-scanner   # stop after scanning
go run ./cmd/fetchtestdata               # the engine and the test corpora, into a checkout
go run ./cmd/ml1 --engine                # where the LOWL source is looked for, and what answers
go run ./cmd/ml1 --fetch-engine          # install the engine for a user with no checkout
go run ./cmd/maclo --engines             # what engines this build has compiled into it
```

`lasm` flags also come from env vars (`LASM_` prefix) or a JSON file via `--config`, courtesy of
`peterbourgon/ff`. `ml1`'s own options are hand-parsed, because the operating instructions let
options and input files interleave; the long `--` options are all extensions.

## Golden tests

The ML/I processor itself lives in `pkg/ml1`. `cmd/ml1` is a thin wrapper over `ml1.Run`, and the
golden tests drive `ml1.Run` **in process, from buffers** — never by exec'ing the built binary.
Adding behaviour to the engine means adding it there, not in `cmd/ml1`.

There are two corpora, and they are held to different standards on purpose:

- `pkg/ml1/testdata/local` is ours, committed, compared **byte for byte**, and run with
  `DebugWidth: NeverWrap`. Write the inputs yourself from the manual, then produce each `.out` with
  the oracle — **ML/I on Apple (Intel) under macOS, version 4.13 (CKQ)**, kept at `.downloads/ml1`,
  which self-reports as `ml1: macOS version 4.13 (CKQ)` — and **read it before committing**. Other
  ports of ML/I do not all agree byte for byte, so `TestOracleIdentity` pins its sha256 and version
  string whenever the binary is present. The input is ours, so
  the output is our own text and nothing of upstream's ends up in the file. Never commit a `.err`
  here: a clean run writes nothing to the debugging stream, and the harness reads a missing `.err`
  as "must be empty". A `.lst` golden, the listing `S20` controls, works the same way and only the
  `listing` case has one. Whitespace is significant; the directory's `.editorconfig` disables
  trailing-whitespace trimming and `cat -e` is how you check. See its `README.md` for the traps.
- `testdata/upstream/tests-ac` is the suite from ml1.org.uk, fetched with
  `go run ./cmd/fetchtestdata`, gitignored, compared with **`diff -b` semantics** and
  `DebugWidth: 72` because that is what its own `runtest.sh` does. Never commit anything from it,
  and never relax the comparison to make a case pass.

There is a third suite with no golden files at all: `TestExamplesAgainstOracle` runs the example
programs in `testdata/` (gitignored, from Rosetta Code) through `ml1.Run` and through the oracle and
requires them to agree byte for byte on both streams. It skips without the oracle or the examples,
its expectations are a table in `pkg/ml1/examples_test.go`, and an example in the directory that the
table does not name is a failure. `testdata/run-examples.sh` runs the same list against the built
binary; keep the two in step.

Both corpora skip when the engine has nothing to run — `ml1.Run` reports `ml1.ErrNoEngineSource` because the
LOWL source of ML/I is not in this repository and is not on this machine either. It lives in
`.downloads/lowlml1/ml1ajb.lwl`, which `cmd/fetchtestdata` downloads along with the corpora. The
skip is keyed on the sentinel so it expires by itself; do not add an environment variable or a
build tag to gate it, or it will become permanent. A skip and a failure are different things; check
which you are looking at.

**The local corpus passes and the upstream corpus does not.** Every local case matches byte for
byte. Upstream, the output streams match except for `macrotim`, and the debugging streams
differ everywhere: the engine is the 1986 LOWL source and the golden files came from the 2006 C
implementation, whose error print-outs are laid out differently. That is the version skew
`docs/explanation/running-ml1-on-the-lowl-vm.md` describes, and it is diagnostic rather than a gate.
Do not chase it by editing the comparison.

`go test ./pkg/ml1 -update` rewrites the local corpus only; the upstream corpus skips under that
flag rather than being rewritten. It also refuses to *create* a golden that does not already exist,
so current behaviour cannot become the specification by accident. `go test ./... -update` fails
elsewhere with "flag provided but not defined", so name the package.

The root `.gitignore` deliberately **does not** ignore `*.out` — the golden files use the upstream
`.out`/`.err` names. Do not add that rule back. `.gitattributes` pins `*.ml1`, `*.out`, `*.err`, and
`*.lst` to `eol=lf`, which is what makes byte-exact comparison portable.

See `docs/reference/golden-tests.md` and `docs/explanation/upstream-test-suite.md`.

**`go test ./...` fails on purpose**, in one place: `TestGoldenUpstream`, on the version skew above.
Nothing else should be red, so treat any other failure as a regression. `pkg/lowl/vm/op_test.go`
used to hold `t.Errorf("%s: not tested")` markers as a TODO list and no longer does — every opcode
has a case. If a new opcode arrives without one, add the placeholder back for it rather than leaving
the gap silent, and replace it with a real test rather than deleting it.

## Pipeline

`cmd/lasm` wires the stages together in `run()`:

```
source file
  → scanner  (pkg/lowl/scanner)   bytes → tokens
  → cst      (pkg/lowl/cst)       tokens → one node per source line, errors carried on the node
  → ast      (pkg/lowl/ast)       cst nodes → Op + typed Parameters
  → assembler(pkg/lowl/assembler) ast → *vm.VM with Core[] populated
  → vm.Run   (pkg/lowl/vm)        executes Core[] until HALT/MDQUIT
```

Errors are handled differently at each stage: the scanner and cst embed errors in tokens/nodes and
keep going (so `lasm` can report several at once); ast and the assembler bail on the first error.

`lasm` writes debug artifacts to the **current working directory** — `scanner_buffer.txt`,
`scanner_tokens.txt`, `ast_listing.txt`, `asm_listing.txt`, `asm_symtab.txt`, `vm_stdout.txt`,
`vm_stdmsg.txt` — so run it from a scratch directory. They are all opt-in, because `ml1.Run` uses the
same stages in process and must write nothing and print nothing: `assembler.Assemble` takes an
`Options` with `Trace` and `Listings`, `cst.ParseBuffer` is the entry point that has no file to name,
and the machine's own commentary goes to `vm.Streams.Trace`.

Two tests keep it that way, and a new stage has to satisfy both. `TestRunWritesNothing`
(`pkg/ml1/silence_test.go`) runs the engine from an empty directory with the process's own streams
redirected, and requires the directory to still be empty and both streams silent.
`TestDebugArtifactsAreDeclared` (`debug_artifacts_test.go`, at the top of the repository) reads the
source under `pkg`, `cmd` and `internal`: only the functions in its `writeSites` table may call
`os.Create`/`os.WriteFile`/`os.OpenFile`/`os.Mkdir*` outside `cmd`, nothing outside `cmd` may touch
`os.Stdout`/`os.Stderr`/`os.Stdin`, and every artifact name written as a literal must be in its
table **and** in `.gitignore`. Adding an artifact means adding it in all three places. `internal` is
scanned on the same terms as `pkg` on purpose — a package that only had to move one directory to
escape the rule would not be covered by it.

## Opcodes are the spine

`pkg/lowl/op.Code` is the single enum shared by scanner, cst, ast, assembler, and vm. Adding or
renaming an opcode means touching, in order:

1. `pkg/lowl/op/codes.go` — the enum
2. `pkg/lowl/op/stringer.go` — hand-written `String()` (no `go:generate` here)
3. `pkg/lowl/op/lookup.go` — hand-written `Lookup()` map, which is what makes the scanner recognize
   the mnemonic (`scanner/opcodes.go` just delegates)
4. `pkg/lowl/assembler/assemble.go` — a `case` in the big switch, grouped by argument *shape*
   ("OP", "OP VARIABLE", "OP LABEL FLAG(A|X)", …) rather than alphabetically
5. `pkg/lowl/vm/step.go` — execution
6. `pkg/lowl/vm/op_test.go` — a case (a `not tested` placeholder at minimum)

Codes at the end of the enum prefixed `MD` (`MDERCH`, `MDLABEL`, `MDQUIT`) plus `GOTBL`, `NOOP`, and
`UNKNOWN` are implementation-dependent: they are synthesized by the assembler and rejected if they
appear in source.

## Assembler

Single pass with back-fill. `symbolTable` (`symtab.go`) holds four kinds of symbol — `address`,
`alias`, `constant`, `literal` — plus `undefined` for a forward reference. `AddReference(name, pc)`
queues the core address needing the value; after the pass, `Assemble` walks the table and patches
every queued address. Undefined symbols are reported together at the end, not at first use.

Some opcodes emit no code (`ALIGN`, `DCL`, `EQU`, `IDENT`, `MDLABEL`, `NB`, `PRGST`); `STR` emits one
word per character, with `Source.Continuation` set on the trailing words so listings collapse them
back to one line.

`OF(...)` expressions are the only macro. `assembler/macros.go` splits the text, `pkg/postfix`
converts infix → postfix and evaluates it against the constants in the symbol table.

Multi-exit subroutines assemble into a jump table: `SUBR` records the exit count, and a `GO`/`EXIT`
carrying a `C` or `T` flag becomes `GOTBL` with `ValueTwo` as its jump-table index. Any other opcode
resets the `jumpTable` counter, so the table entries must be contiguous in the source.

## VM

`vm.VM` is a struct of registers plus `Core [65536]Word`. A `Word` is not a machine word — it carries
`Op`, `Value`, `ValueTwo`, `Text`, and the originating source line, so core doubles as the program
listing. Data and code share the same array; a variable is a `CON` word and `directLoad`/
`directStore` read and write its `Value` field.

Reserved low addresses are set up in `vm.New()`: address 0 is `HALT`, followed by `DSTPT`, `FFPT`,
`LFPT`, `PARNM`, `SRCPT`. `Registers.Start` comes from the `BEGIN` label; the assembler warns and
leaves it pointing at the address-0 `HALT` if `BEGIN` is missing.

`Run` is capped at 10,000 cycles and returns `ErrCycles` on overrun — bump it in `run.go` when
running real ML/I logic. `ErrQuit` (from `MDQUIT`) is the graceful exit and is swallowed by `Run`.

`TestLOWLTEST` (`vm/lowltest_test.go`) is the only end-to-end test of the kernel: it assembles and
runs LOWLTEST L4A from `.downloads/lowltest/ltestl4a.lwl`. That file cannot be committed, so the
test **skips** when it is absent — `go run ./cmd/fetchtestdata -corpus lowltest` turns it on. If you
touch opcode execution, make sure this one ran rather than skipped.

## Two front ends

`cmd/ml1` implements Appendix AA — single letter options, options and input files interleaved, the
engine found on disk — and is **not free to be improved**: being a drop-in for the reference
implementation is the whole of what it is for. `cmd/maclo` has no such obligation, takes ordinary Go
flags, and runs an engine compiled into the binary. Anything better goes in maclo. Both are thin
wrappers over `ml1.Run`, and behaviour added to either rather than to `pkg/ml1` is behaviour nothing
tests.

## Where the engine comes from

Two answers, and `Job` has a field for each. `LOWLSource` is a path and `Engine` is the name of a
source built into the binary; `LOWLSource` wins if both are set, and with **neither** set `Run`
searches the file system and *does not consult the embedded engines at all*. That last rule is what
keeps cmd/ml1 behaving the way its operating instructions say whatever the binary was built with,
and `TestEmbeddedIsNotConsultedByDefault` is what stops it drifting. maclo fills `Engine` in;
ml1 leaves both empty.

`pkg/ml1/embed.go` holds the embedding. `//go:embed engines` points at `pkg/ml1/engines/`, whose
`.gitignore` denies everything and allows back only itself and a `README.md` — the licence permits
building the source in but not redistributing it, so a `.lwl` must never be committable. **The
tracked README is load-bearing**: `//go:embed` on a directory with only hidden files fails to
compile, and a tree with zero engines has to build. `Engines()` lists them newest first by file
name, which is where ML/I's version lives, and the newest is `DefaultEngine()`.

`pkg/ml1/engine.go` owns the file search. `EnginePaths()`
returns, in order: `$ML1_LOWL_SOURCE`, then `$ML1_HOME` or the per-user directory (macOS and Windows
follow `os.UserConfigDir`, elsewhere `$XDG_DATA_HOME/ml1`), then `.downloads/lowlml1/ml1ajb.lwl`
relative to the working directory. `Job.LOWLSource` overrides all of it. `cmd/ml1` deliberately does
**not** read the environment itself, so that a program embedding the library finds the same file the
command does.

`internal/fetch` does the downloading and digest checking: `cmd/fetchtestdata` populates a checkout
— and `InstallEngines` copies the `.lwl` on into `pkg/ml1/engines/`, which is the step that turns
fetched material into a build input — while `ml1 --fetch-engine` installs the engine alone for a
user who has no checkout. It writes only where its caller names, prints nothing (progress goes to
`Options.Progress`), and refuses any destination inside a git working tree that is not covered by a
`.gitignore` whose first line is `*` — a directory in no repository at all is fine, which is what
the per-user install relies on.

## Repository conventions

- Every `.go` file starts with the three-line `ml_i - an ML/I macro processor ported to Go` /
  copyright header. Keep it on new files.
- `testdata/`, `.archive/`, `.notes/`, `.downloads/` each ignore their entire contents (`*` plus
  `!.gitignore`). This is deliberate: LOWL/L sources and documentation are copyright P.J. Brown and
  R.D. Eager, and their license forbids redistributing machine-readable copies. Do not commit ML/I
  sources, test inputs derived from them, or excerpts of the upstream docs into tracked files.
- Standard library only, apart from `peterbourgon/ff/v3` for CLI config in `cmd/lasm`.
