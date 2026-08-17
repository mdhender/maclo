# The L front end

`pkg/l` scans, parses and name-checks **L**, the machine-independent language the logic of ML/I is
written in. It does not compile: there is no back end behind it. `cmd/macl` and `cmd/lcheck` drive
it.

This is the second of the two ways to port ML/I that
`docs/explanation/running-ml1-on-the-lowl-vm.md` describes. The first — implement LOWL and run the
distributed source unchanged — is what `pkg/lowl` and `pkg/ml1` do and is the route this repository
took. This is the front half of the other one.

## Commands

There are two, and the split is the one `cmd/lasm` and `cmd/maclo` already have: a tool for working
on the implementation, and a tool for working on a program.

**`macl` reports on a program.** It is the one that will grow a back end, and `macl run` reserves
the word for it.

```sh
go run ./cmd/macl check   file.l    # scan, parse and resolve; report what is wrong
go run ./cmd/macl summary file.l    # count what the front end saw
go run ./cmd/macl list    file.l    # the statement listing, indented by nesting
go run ./cmd/macl symbols file.l    # the symbol table
go run ./cmd/macl source  file.l    # the program back as L, which is what the round trip checks
go run ./cmd/macl run     file.l    # not yet; it says so, and says what to use instead
```

Each takes `--out PATH` (`-` is the standard output, and the default), `--max-errors N` (0 for all)
and `--quiet`, and options may come before or after the file name. Diagnostics go to the standard
error in source order. The exit status is **0** clean, **1** the source had errors, **2** the command
line was wrong or a file would not open, and **3** from `run`, which is the one code that means the
program was fine and macl was not.

`run` is not a stub that refuses to look at the file. Everything short of code generation works, so
it runs the front end first: a program that will not resolve gets that answer and exit 1, and only a
clean one reaches the missing half.

**`lcheck` reports on the parse.** It dumps the stages, the way `lasm --test-scanner` does for LOWL,
and it is what you reach for when the front end itself is what is wrong.

```sh
go run ./cmd/lcheck --source file.l                      # diagnostics only
go run ./cmd/lcheck --source file.l --listing -          # and the statement listing
go run ./cmd/lcheck --source file.l --tokens - --cst -   # the earlier stages
```

And the two that are neither:

```sh
go run ./cmd/fetchtestdata -corpus lml1                  # fetch the L source of ML/I
go test ./pkg/l -update                                  # rewrite the golden files
```

Everything a command prints about a program comes from `l.Summary`, in `pkg/l`, rather than from a
walk written in `cmd`: a count computed in a command is a count nothing tests, and `TestML1AIE`
asserts every field of that struct against the real 2,510 lines.

## The stages

```
source file
  → scanner  (pkg/l/scanner)   bytes → tokens
  → cst      (pkg/l/cst)       tokens → one node per source line, arguments as a tree
  → ast      (pkg/l/ast)       lines → a nested tree of typed statements
  → sema     (pkg/l/sema)      the structural rules, then the names
```

`pkg/l.Parse(src []byte) *Result` runs all four from a buffer. Every stage **accumulates**
diagnostics and recovers at statement granularity, unlike `pkg/lowl/ast`, which bails on the first
error: the deliverable is a listing you read over a 2,500-line file, and one diagnostic per run
means one typo fixed per run.

Nothing under `pkg/l` opens a file or touches a process stream. Every listing takes an `io.Writer`,
and the only calls to `os.Create` are in `cmd/macl` and `cmd/lcheck`, each on a path the user named
— so `pkg/l` appears nowhere in the `writeSites` table in `debug_artifacts_test.go` and the root
`.gitignore` needs no new entries. Both commands have a test that runs every subcommand from a
temporary directory and requires it to be empty afterwards.

## Where it differs from the LOWL front end, and why

`pkg/lowl` is the model, and three things about it are deliberately not copied.

**The ast is a tree, not a list.** `pkg/lowl/ast` is `Node{Op, Parameters}` — one flat node per
source line — because LOWL has one opcode per line and no nesting. L has five paired constructs
(`SECTION`/`ENDSECT`, `BLOCKDEC`/`ENDBLOCK`, `SUBROUTINE`/`ENDSUB`, `IF`/`END`, `CHAIN FROM`/`ENDCH`),
real expressions, and statements whose shapes have nothing in common. So there is one Go type per
statement, compound statements hold their bodies, and the closers get no types at all: each is
folded onto the statement it closes.

**`pkg/l/stmt` is one table, not three lists.** `pkg/lowl/op` keeps an enum, a hand-written
`String()` and a hand-written `Lookup()` map in step by hand. Here the table is indexed by the enum,
`String` and `Lookup` both read it, and `TestTableIndexMatchesKind` is what makes that safe. The
table also carries which SECTION classes each statement may appear in, so `sema` gets the placement
check without a switch of its own.

**The scanner does not classify keywords.** `pkg/lowl/scanner` calls `op.Lookup` while it scans, so
its lexer has to be edited whenever the language changes. Here every identifier-shaped run comes out
as a `Word` and the parser decides from position. L makes that safe: keywords are 2–11 characters
and identifiers 3–6, so the short keywords can never be identifiers.

The `cst` *is* copied: it is flat, one node per source line, because nothing in L spans a line.
Pairing the openers and closers is a stack walk and it happens in `ast/build.go`. Putting it in the
cst would cost something specific — the cst would have to know that `THEN` at the end of a line
opens a block while `THEN GO TO X` does not, and that is a fact about the `IF` statement that
belongs in one place.

## The lexical traps

Each was verified against the real L source of ML/I.

| trap | why it bites |
|---|---|
| Comment terminators are two characters | `//ML/I AUG 30 1971//` is line 2 of the corpus. Stopping at the next bare `/` breaks immediately. |
| Quotes are recognised before comments | One `CHARMATCH` has `'*'` and `'/'` arms on one line. |
| `[` is context sensitive | `[SKIP] SETSW …` is a label and `PRTEXT[SKIP]` is a string. Identical bytes; only the preceding word differs. This is the scanner's one lexical decision, and one slot of state. |
| `//` is sometimes an argument | After the comma on `SECTION` and `BLOCKDEC`, and after `EXIT` on `SUBROUTINE` — that last with no comma. Everywhere else it is trivia. |
| Prefixes sit on either side of the label | The corpus writes `[BEGIN] /-IN-/SET` and `/-CSS-/[FNCTEX] CALL`, always with no interior spaces. The manual shows neither. |
| A prefix can follow `THEN` | `IF NEGVAL = 1 THEN /-OVP-/SET …`. It belongs to the `SET`. The manual documents neither the position nor the case. |
| Layout is insignificant | `STACK IDPT (PT)` and `STACK PARSW(SW)` must parse the same way, so nothing folds `NAME( )` into one node before a statement's grammar asks for it. |
| Bracket whitespace is significant | `PRTEXT[)    ]` carries four spaces. |

## What `sema` checks, and what it does not

It resolves names. It does **not** type check: a variable's type is inferred from the last two
characters of its name and recorded, and nothing is enforced with it. The manual's operand rules are
subtler than the suffix — a constant may appear only after a relational operator, an indirect
address after one is always through a pointer — and a half-done type check is worse than none.

Checked: where each statement may appear; the five closers matched and their names equal;
`RETURN FROM`/`EXIT FROM` naming the enclosing routine; `EXIT FROM` only where the subroutine
declares one; `LINK BACK` only in a linkroutine; `*` and `/` only inside `OF`; `&` and `|` only in
`SETSW` and a condition join, never mixed; `BLOCK( )` only in the block moves; `RL` and `LID` only
as `DC`/`OPMAC` arguments; every name declared once and used as the right kind of thing; and a
`CALL` supplying an argument exactly when the callee declares a parameter, and `EXIT` exactly when
it declares an exit.

Warnings rather than errors: an identifier outside the manual's three to six characters (the
language breaks its own rule — `STOPCODE` is eight, and so are two SECTION names); a declared
variable nobody uses; a label nobody branches to; a `SECTION` name that is not one of the ten; a
block `IF` inside a block `IF`.

**Names the language defines for itself** are seeded in `sema/predefined.go`, each with its citation:
the constants and markers, the six length macros, the machine-dependent labels and subroutines with
their shapes, and `BEGIN`/`MBEGIN`, which the initialisation code branches to. Two are conditional
or derived rather than listed:

- `ERBLOC`, `GHSHTB` and `SVEC` are generated by `HETABLES` and written nowhere. They are seeded
  **only when a `HETABLES` is present**, which keeps the allowance checked rather than silent.
- A block's size constant is its name with `SZ` appended, derived from the `BLOCKDEC`. The manual
  lists four and the logic of ML/I declares five, so a list would have been wrong.

## Tests

`pkg/l/testdata` is ours, committed, and compared byte for byte — eleven programs covering the
traps above and seven more covering one class of diagnostic each. See its `README.md`. A missing
`.err` or `.sym` means "there must be none", and the harness **refuses to create a `.lst` that does
not already exist**, so current behaviour cannot become the specification by accident.

`TestRoundTrip` renders every case back to L, reads it again, and requires the same text. The
listing is a canonical re-render rather than an echo of the input, so a golden diff reports a change
in the parse rather than in the file's whitespace — and the round trip is what stops the canonical
form from quietly losing something.

`TestML1AIE` runs the whole front end over `ml1aie.l`, the real 2,510-line L source of ML/I. It
**skips** when the file is absent, keyed on the file rather than on an environment variable, so the
skip expires by itself once `go run ./cmd/fetchtestdata` has run. It asserts the count of every
statement, the ten SECTIONs in order, the two nesting restrictions the manual states as facts, a
byte-identical round trip, and **exactly one undefined name**.

That one is a real defect in AIE rather than a gap here: a `TEST` branches to a label whose
declaration is spelt with a letter too many. The LOWL distribution settles which of the two is the
typo — it spells the label the way the branch does. The test asserts the count and does not name the
symbol, and there is deliberately no known-defects allowlist: a mechanism for excusing corpus errors
is a mechanism for hiding regressions.

## Where the L source comes from

`ml1aie.l` is copyright P.J. Brown and R.D. Eager and may not be redistributed, so it is not in this
repository. `internal/fetch/manifest.json` has an `lml1` entry that downloads it from
<https://www.ml1.org.uk/tgz/lml1.tar.gz> into `.downloads/lml1/`, which is gitignored. The entry
carries no `engine` flag: that mechanism is for the `.lwl` sources `//go:embed` compiles into the
binary, and nothing embeds L.

The L manual is *Implementing software using the L language*, second edition, at
`.references/lmap.txt`. Every citation in the code and in this page points into it.
