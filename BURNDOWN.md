# Burndown

A working list of what is left, and of what only looks like it is left. Measured on 2026-08-15
against `e2df3dd`, and revised the same day as items 5, 6, 1, 7 and 2 closed. Items 3 and 8 closed
on 2026-08-16, which empties the list: every numbered item below is either done or, in the case of
item 4, a diagnosis to keep rather than a task. What is left is the AJB/CKQ version skew, and
closing that means implementing CKQ's changes, which is a different project from porting the source
that exists.

Background reading, in this order, before picking anything up:
`docs/explanation/running-ml1-on-the-lowl-vm.md`, then `pkg/ml1/testdata/local/README.md` for the
rules that govern the corpus, then `docs/reference/golden-tests.md`.

## Where the port stands

| suite | result |
|---|---|
| `TestGoldenLocal` | **25 of 25 pass**, byte for byte |
| `TestGoldenUpstream` | 1 of 11 passes (`overflow`); the other 10 fail on version skew |
| `TestExamplesAgainstOracle` | **17 of 17 pass**: 15 agree with the oracle byte for byte, 2 differ as they are required to |
| everything else in `go test ./...` | green |

`TestGoldenUpstream` is the only expected failure. Anything else red is a regression.

All three suites drive `ml1.Run` in process, from buffers. The built `cmd/ml1` binary agrees with it
— checked by hand against the upstream suite, and by `testdata/run-examples.sh` over the examples —
but the binary is not what `go test ./...` exercises, so behaviour added to `cmd/ml1` rather than to
`pkg/ml1` is behaviour nothing tests.

## What the upstream failures actually are

All ten are the AJB/CKQ version skew. This was re-derived rather than taken on trust, because
`got ""` in the failure output reads like truncation rather than like a layout difference. It is
not truncation. The debugging streams of `alter`, `errors`, `escalate`, `names`, `override`, `s`,
`skipins` and `structur` differ, their output streams do not, and for seven of the eight *every*
difference is a **surplus blank line**. All of those come from three literals in the source we run:

```
ml1ajb.lwl:3986  [ERPSRC]  MESS 'source text$$$'
ml1ajb.lwl:4001            MESS '$$$Error(s)$'
ml1ajb.lwl:4407  [LOHALT]  MESS '$$At end of process: '
```

CKQ has one `$` fewer at each site. The message text either side is byte-identical — an `MCNOTE`
run differs from the oracle by exactly one trailing newline and nothing else. So the port
reproduces its source correctly and the golden files record a later one.

The eighth, `errors`, has five diff lines that are not blank: the second category the repo doc
records, spacing in the "with arguments" context display, where the two versions show different
amounts of text either side of the `  ---  ` marker. Read a whole diff before concluding a case is
all one thing; this one is not.

The remaining two are missing features of the 1986 source, not defects:

- `strings` uses `MCCVAR`, which does not appear in `ml1ajb.lwl` at all (`grep -c` returns 0). The
  construction is echoed literally, each use raises an error, and the `S12` quota runs out, which
  is why this case fails with `ErrDebugQuota` rather than a diff.
- `macrotim` line 26 uses the bitwise `&` and `|` of a macro expression. `GETEXP` accepts
  `+ - * /` and rejects the rest as an illegal argument.

Only the LOWL source for AJB is published (`.downloads/lowlml1/MANIFEST` lists that one file), so
this cannot be closed by fetching a newer engine. It closes only if someone implements CKQ's
changes, which is a different project from porting the source that exists.

## Open items

### 1. ~~The debugging stream has no byte-exact coverage~~ — done

No case in the local corpus wrote anything to the debugging stream. All 21 relied on
`optional: {".err": true}`, which asserts the stream is *empty*, so the whole diagnostic path was
untested where we can compare it: error messages, `MCNOTE`, `MCWARN`, the `S18` end-of-process
report, and the `S12` quota. It was exercised only upstream, where the comparison is against the
wrong version.

The obvious fix — commit `.err` goldens to the local corpus — is **ruled out on purpose**, and the
rule is worth reading before arguing with it. `pkg/ml1/testdata/local/README.md` keeps `.err` out
because the error wording is upstream's expression, and a golden file would be a machine-readable
copy of it. The sanctioned pattern is a substring assertion in an ordinary Go test;
`pkg/ml1/storage_test.go:79` was the existing example, checking for `"Error(s)"` and
`"source text"`.

`pkg/ml1/debug_test.go` extends that pattern. Five tests, each run against the oracle first so that
what is asserted is behaviour rather than what this engine happens to do:

- `TestDebugNoteWithoutContext` — `MCSET S4=1`, the one **byte-exact** assertion this stream allows.
  `S4` suppresses the context print-out, so all that reaches the stream is `MCNOTE`'s argument
  between two newlines, and none of it is upstream's wording. It also pins that an operation macro
  consumes its own newline, so three lines of `MCSET` and `MCNOTE` produce no value text at all.
- `TestDebugNoteWithContext` — `S4=0`: the note, then `detected in` / `called from` walking out
  through the macro to the source text. `MCNOTE` does not move `S5`.
- `TestDebugWarningMarker` — `MCWARN`, the message a marker that named nothing produces, one `S5`
  per occurrence, and `S3=1` suppressing **the message only**: the value text is the same either
  way, which a suppression that also changed the copy would not be.
- `TestDebugErrorAbortsTheConstruction` — an error inside a macro. The `Error(s)` prologue that
  `S5` actually counts, the context chain, the `aborted due to above error` subsidiary message, and
  the fact that the scan carries on: the rest of the replacement text and the rest of the source
  both reach the output.
- `TestDebugEndOfProcessReport` — `S18` at 0, 1, 2 and 3, each bit alone and both together, and the
  rule that the report is not charged against the `S12` quota (`S12=0` still gets it written).

The substrings are chosen to steer around the two skewed regions, so none of this pins the version
skew. The `S4=1` slice is exact because the context print-out is the only part that goes through
`ERPSRC`, the site of one of the three skewed literals — which is what
`testdata/hello-stderr.ml1` and `run-examples.sh` had already shown.

Two things turned up while writing it, both worth having recorded:

- **`S18=3` writes the two parts in the opposite order to CKQ.** `ml1ajb.lwl:4407` has `LOHALT`
  print `At end of process: ` and *then* `GOSUB PRENV` to list the constructions. Appendix AA
  describes the other order — the statistics "preceded by a list of the currently defined
  constructions" — and the oracle does that. A third instance of the same skew, in a place none of
  the upstream cases reach because their prelude sets `S18=2`. The test asserts the AJB order
  deliberately, so that changing it has to be a decision.
- **The `S12` diagnostic is ours, not ML/I's.** See item 7.

### 2. ~~Nothing in `testdata/` can be promoted as it stands — rewrite instead~~ — done

This was an open question in the first draft of this file and it is now settled, against
promotion. Every example in `testdata/` comes from Rosetta Code
(<https://rosettacode.org/wiki/Category:ML/I>), including the three that were already there:
`100-doors.ml1`, `99-bottles-iterative.ml1` and `99-bottles-recursive.ml1` are
**character-identical** to the versions on the site. The ML/I entries there are largely Bob Eager's,
the same author as the oracle and ml1.org.uk, under CC BY-SA / GFDL.

`pkg/ml1/testdata/local/README.md` requires inputs to be original work written from the manual, so
that the repository carries no third-party text. Copying any of these in would break that, and the
directory they are in now is gitignored precisely so that they do not.

What transfers is the *constructs* — the local README already says macro semantics are not
copyrightable and only a transliteration would be. So the item is: read an example, note what it
exercises, and write an original case covering the same ground. In rough order of what the corpus
is missing:

- ~~**`UNLESS`.**~~ Done, in `loop`'s `COUNTUP` — see item 5.
- ~~**A permanent variable subscripted by a temporary one**~~ — `arrays`. `pvars` covers
  subscripting by a *permanent* variable (`PP1`, `PPP2`); `PT1` is the composition that makes
  `MCPVAR n` behave like an array with a moving index, because the array is global and the index
  belongs to the call. The last example is the one that earns the case: a walk of the array from
  inside another walk of it, which an index kept in a permanent variable would not survive.
- ~~**`MCSET S10` stream switching**~~ — `streams`, and it needed harness work; see below.
- ~~**`GE`, and the full `EN`/`GR`/`GE` vocabulary in one case**~~ — `jumps`, all three at the
  same boundary so that the only difference between them is visible in one place.
- ~~**`MCGO LP102`**~~ — `jumps` again: `MCGO LT1` is a jump table indexed by a variable, and
  `MCGO L T1 + 10` gives the table a base.
- ~~**A macro with no delimiter structure**~~ — `define` gained the zero-argument form, which is
  what that case was already about.
- ~~**`MCSKIP T,` — the non-nestable skip**~~ — `straight`. `skipopts` used `MCSKIP T,` without
  ever showing it fail to nest. The third example is the subtle one, out of §2.7.3: a straight skip
  met while scanning a matched one is matched first, so a `>` it swallows does not close the outer
  skip.
- ~~**Recursion deep enough to matter**~~ — `pkg/ml1/recursion_test.go`, a Go test as the first
  draft of this item said it should be. `storage_test.go` already covered running *out* of
  workspace; what was missing was the other side, that a deep recursion unwinds correctly and that
  the workspace is what limits it. The macro writes a bracket on the way down and its match on the
  way up, so a correct result is only possible if the whole stack was held and then unwound — a
  countdown printing on the way down would pass with the unwinding broken. Depth 200 with 24000
  words, depth 200 with the default 5000, and depth 50 with the default, all three run against the
  oracle first. The boundary is **not** pinned: exhaustion depth is implementation dependent, which
  is why upstream's `overflow` is in the `unstable` map. That both implementations happen to fail at
  12000 words and succeed at 20000 is recorded in the test's comment and relied on by nothing.

Two things worth having recorded, both found by the oracle rather than reasoned out:

- **The local corpus can now have more than one input stream.** A case may have `NAME.2.ml1`
  through `NAME.5.ml1` beside it; `casesIn` skips them as cases and `extraStreams` feeds them in
  order after the case's own file, so `NAME.ml1` is stream 1 and `NAME.2.ml1` is stream 2. The
  numbering only works because this corpus has no prelude — the upstream one does, which would make
  its case file stream 2 — so the harness fails loudly on the combination rather than misnumber it.
- **A structure representation is evaluated, so re-reading a definition is not idempotent.**
  §5.2.4 gives `MCDEF`'s order of evaluation as {arg A}, {arg C}, {arg B}: the name comes last and
  is evaluated like any other argument. The first draft of `streams` put `MCDEF GREET AS <the
  definition has arrived>` in the stream it then rewound, and on the second pass `GREET` expanded
  while ML/I was working out what to define, so what got defined was a macro named `the` with
  delimiters `definition`, `has` and `arrived`. The case was rewritten to keep its definitions in
  the stream that is not rewound; the trap is in the corpus README, because the manual's own
  motivation for the rewind is a set of macros needing multiple passes and this is what that walks
  into.

`testdata/README.md` has the full inventory and what each example covers.

### 3. ~~Wire `testdata/run-examples.sh` into something that runs regularly~~ — done

The 17 examples were a differential suite against the oracle that nothing ran. They are worth more
than their size suggests: they found the two items below, and 15 of the 17 agree with the oracle on
both streams, which is a much broader statement about the engine than the 25-case local corpus makes
on its own.

The open question was whether a suite whose inputs are not in the repository earns a place in
`go test ./...`. **It does**, and the deciding argument is that the alternative is worse than a
skip: a check nobody remembers to run is not a check. The pattern is already in use twice — the
upstream corpus skips on its directory, `TestOracleIdentity` skips on the binary — so this adds no
new kind of conditional coverage, only one more instance of it.

`TestExamplesAgainstOracle` in `pkg/ml1/examples_test.go` runs all 17 in **0.6s**, driving `ml1.Run`
in process from buffers rather than exec'ing the built binary, which is what makes it a test of the
engine and not of the command line. It skips on the oracle, skips on an empty `testdata/`, and skips
each example whose files are missing.

The script stays, and is no longer redundant: it is now the only thing exercising `cmd/ml1` over
these programs — the flags, the files, and the streams they are wired to — which the burndown's own
note about behaviour added to `cmd/ml1` says is worth having. Its header and `testdata/README.md`
both say to keep the two lists in step.

Four things the design settled, each verified by breaking it and watching the test fail:

- **Both streams, byte for byte** (`equalExact`, not `diff -b`). Both sides come from the same two
  programs on the same machine, so there is nothing to be tolerant about. Running at
  `Workspace: 2000` was the check: it produced a real diff on the output stream of six examples and
  on the debugging stream too.
- **A table, not a directory walk.** An example may read a second input stream and only the table
  knows which file that is. So an `*.ml1` in `testdata/` that the table does not name is a
  **failure** — putting a program there and forgetting to wire it up is exactly the state this item
  closed, and it would otherwise come back silently.
- **The two skew cases must differ.** `bitwise-operations` and `csv-to-html` carry a `skew` note and
  are asserted to disagree; agreement fails the test and says the construct has arrived and the note
  is stale. This is item 4's "cheapest way to confirm the diagnosis has not drifted", made
  automatic.
- **The oracle's exit status is ignored.** An example that raises errors ends non-zero and that is
  not the comparison; only what it wrote is.

### 4. Two minimal reproductions of the upstream skew are now available

`bitwise-operations` and `csv-to-html` reduce the two non-newline upstream failures to small,
readable cases:

- `bitwise-operations` is `&` and `|` in a macro expression, which the Rosetta page notes are
  "available from version CKD onwards". Fifteen lines, versus finding it on line 26 of `macrotim`.
- `csv-to-html` is `MCCVAR`, absent from `ml1ajb.lwl` entirely. It fails with
  `Argument 1 has illegal value, viz "C1"`, which is a far clearer symptom than `strings` running
  the `S12` quota out.

They are now watched rather than merely available: item 3 asserts that each one **differs** from the
oracle, so `go test ./...` reports it the day either feature arrives and the diagnosis here goes
stale. If one is implemented, these are still the cases to check first.

### 5. ~~Correct the note in `pkg/ml1/testdata/local/loop.ml1`~~ — done

The case used to say:

> Iteration is expressed as recursion, because a label search runs forward only.

Both halves needed work, and the manual settles it. Section 5.4.3, "Exact description of a
'goto'": a designated label already **present in the current environment** is jumped to directly,
and only a label that is not provokes a forward search from the point of scan. Section 2.6.7: a
label is added to the environment as it is passed. So a backward jump inside a replacement text
resolves without any search at all, which is why `99-bottles-iterative` and `100-doors` loop.

The real restriction is in the same section's notes, and it is about the *source text*, not the
search: "If it is desired to achieve the effect of a backward 'goto' in the source text, then the
required loop must be defined as the replacement text of a macro call." That is why a loop has to
live in a macro — not because iteration must be recursion.

Corrected in `loop.ml1` and in the corpus README. One thing the first draft of this item got wrong:
it predicted the golden would not change. It does — the note is pass-through text, so it lands in
`loop.out`, which was regenerated with the oracle and re-read.

The follow-on — a macro that loops by a backward jump, so that the case is evidence for its own
prose rather than only describing it — is now in as `COUNTUP`, and it takes `UNLESS` off item 2's
list. It counts up with a permanent variable, because the thing recursion gets for free is a
changing argument and a loop has to arrange that itself.

One trap, worth writing down because the first attempt hit it: the trailing `MCGO` needs a newline
before the closing `>` of the replacement text. `NL` is its delimiter, and without one the oracle
reports `Delimiter (NL) of macro MCGO in line 3 of current text not found` rather than looping. The
golden was regenerated and read; the corpus README records the trap.

### 6. ~~LOWLTEST is fetched, documented, and run by nothing~~ — done

`.downloads/lowltest/ltestl4a.lwl` is the LOWL kernel conformance program, version L4A. It was
fetched, cited by `docs/`, mentioned in four files in `pkg/lowl/vm` as the example of a program with
no S-variables, and opened by no `_test.go` at all.

`TestLOWLTEST` in `pkg/lowl/vm/lowltest_test.go` now assembles it and runs it, skipping on the file
the way `requireEngine` skips on the engine, so a clone that has not fetched it is unaffected. It
asserts three things, and each was checked by breaking it:

- **No line begins `+++`.** Every one of the program's sixty-odd failure messages starts that way,
  so one test covers all of them and there is no list to keep up to date.
- **Every announced section reports `OK` or `found`, and there are 14 of them.** The count is the
  part that matters: a machine that stops in the middle writes no failure message, so the `+++`
  check alone would call a truncated run a pass. Confirmed by capping cycles — LOWLTEST finishes in
  something between 1,500 and 2,000, and at 1,500 the test fails.
- **The character comparison actually compares.** The last section writes
  `ABCDEFGHIJKLMNOPQRSTUVWXYZ 0123456789` and `.,;:()*/-+=` + tab + `"` a byte at a time, then
  prints a literal saying what they should have looked like, for a human to check. The test does
  the check. This is the only part of LOWLTEST that is about how characters are represented rather
  than how control flows.

Worth having because of what it is: the only end-to-end test of the opcode kernel. `op_test.go`
tests opcodes one at a time against hand-built expectations; LOWLTEST runs a real program that
exercises them against each other, and it is the thing that would notice a subtle regression in,
say, `GOADD` or the stacks. It is not proof of much on its own — `fac4cd1` fixed two bugs, `RL`
storing an address instead of a distance and `CSS` popping one link instead of clearing the stack,
that had survived both a clean assembly *and* a passing LOWLTEST — but that is a reason to keep it
running rather than a reason to doubt it.

### 7. ~~The `S12` quota boundary — and its diagnostic is the wrong one~~ — done

Both halves are closed, and the second turned out to be the larger of the two.

The **boundary** — the line on which the quota goes negative and the process is abandoned — was
reached only by accident, when `strings` blew up on the `MCCVAR` skew. `TestFatalQuotaBoundary`
reaches it on purpose, and pins that it is one line further than "reaches zero": with `S4=1` an
`MCNOTE` writes two lines, so `S12=4` covers two messages exactly and `S12=3` does not. In the
second case the message that runs the quota out is still written **whole** — the abort happens on
the newline that ends it. Both halves agree with the oracle.

The **diagnostic** was the wrong one everywhere, not just here. `host.abort` used to write
`err.Error()` straight through `writeMessageText`, so a fatal condition produced a single line
naming a Go sentinel:

```
ml1: S10 has illegal value, viz 9: ml1: input stream has an illegal value
```

What AA.4.1 describes, and what the oracle writes, is an ordinary ML/I error print-out with four
parts: the `Error(s)` prologue, the message, a context print-out saying where the process had
reached, and a line saying it stopped there. Three of those were missing, and so was the count —
the prologue is what §6.3 says `S5` counts, so `res.Errors` was short by one on every fatal path.

The context print-out is the part that cannot be written from Go: `PRCTXT` walks ML/I's own stack of
environments. So `abort` now does what `vm.stackOverflow` does — it writes what it can and **hands
control to `ER7A`**, the point in the source that prints the context and then goes to the
finalisation code. The process ends by running ML/I's own error path rather than by the machine
stopping where the condition was noticed, and the last line is written from `host.finalise` when
`drive` sees `LOHALT` reached. `MCSET S10=9` now produces:

```
Error(s)

S10 has illegal value, viz "9"

detected in
line 3 of source text


Process aborted due to above error
```

which is the oracle's print-out byte for byte, apart from **one blank line** in the prologue: that
is `MESS '$$$Error(s)$'`, one of the three skewed literals item 4 above describes, so the difference
is the one already accounted for. The quota case matches on the same terms.

Four things are worth having recorded:

- **Line 3, not line 2.** `S10` is read afresh before every character, so an illegal value takes
  effect on the character *after* the macro that set it. The oracle says line 3 as well.
- **The quota case has no context print-out**, and that is not an omission: printing one would need
  more lines of the stream than there are. The oracle prints none either. That is why there are two
  entry points, `abort` and `stop`, rather than one.
- **The quota stops being charged once the process is dying**, so that the context print-out cannot
  be cut off by the condition it is explaining.
- **`viz "9"`, not `viz n`.** AA writes the message with the value unquoted; the oracle quotes it,
  the way the MI-logic's own "has illegal value" messages quote theirs.

The messages now match AA's wording exactly, capitals included — `Cannot rewind input stream`,
`Error while writing to output file 2`. AA names no message for a *read* error, so that one follows
AA.4.1.4's shape. The Go error still carries the sentinel and the underlying cause, because that is
what a caller needs and what a reader of the debugging stream does not.

`pkg/ml1/fatal_test.go` covers all four conditions and asserts the parts **in order**, since every
part can be present and the print-out still be wrong. Three of the four were compared against the
oracle first; the rewind failure could not be, because the oracle buffers its standard input and
rewinds it successfully, so it is asserted from AA.4.1.3 alone.

(`ErrNoStorage` is not in this set: it comes from `vm.ErrStackOverflow` in `lowl.go:128` and the
MI-logic has already written its own diagnostic by then, which is what `storage_test.go` checks. It
is also why that process is *not* fatal — `ERLSO` ends through the ordinary finalisation, exit
status 254, where these four end early, exit status 255.)

### 8. ~~Debug artifacts still go to the working directory~~ — done

Carried over from `docs/explanation/running-ml1-on-the-lowl-vm.md`. Every stage writes its listings
to the current directory, which is fine for `lasm` and is what `ml1.Run` must never do. The
`Options`/`Trace`/`Listings` plumbing already existed and was already correct — **nothing was
leaking**, which is why this item was a guard to build rather than a bug to fix.

Two tests, because neither covers the item on its own:

- **`TestRunWritesNothing`** (`pkg/ml1/silence_test.go`) runs the engine from an empty temporary
  directory, with `os.Stdout` and `os.Stderr` replaced by pipes, and requires the directory to still
  be empty and both streams to have received nothing. Four jobs — a clean run, one that asks for a
  listing and the `S18` report, one that reports errors, one that is aborted — plus a job whose
  engine does not exist, which must not create the file it was pointed at either. The assertion is
  "no file at all" rather than "no file called X", which is the only form that catches an artifact
  nobody has thought of yet. Verified by making `assemble` pass `Options{Listings: true}` and print
  a line: it reports both.
- **`TestDebugArtifactsAreDeclared`** (`debug_artifacts_test.go`, at the top of the repository,
  because what it says is about all of it) parses every non-test file under `pkg` and `cmd` and
  applies three rules: only the six functions in `writeSites` may create a file under `pkg`; nothing
  under `pkg` may name `os.Stdout`, `os.Stderr` or `os.Stdin`; and every file name written as a
  literal — a name the tooling chose for itself, since a real output file is named by its caller —
  must be in the artifact table **and** in `.gitignore`. All four failure modes were checked by
  causing them.

The behavioural test is the one that matters and the static one covers its blind spot: a stage that
wrote its listing from a path those four jobs do not reach would still be a library writing files
into a directory it does not own.

It found one real gap and one piece of history:

- **`scanner_buffer.txt` was not in `.gitignore`.** `TestBuffer` writes it for `lasm`, and the six
  other artifacts were named there while it was not. Added.
- **`pkg/lowl/assembler/` still held `asm_listing.txt` and `asm_symtab.txt`**, dated 17:09 on
  2026-08-15. Until `d417033` (17:35 the same day) `assembler.Assemble` took no `Options` and wrote
  its listings on every call, so running its own tests dropped them into the package directory. They
  were invisible to `git status` only because someone had named them in `.gitignore` — which is
  exactly the accident `scanner_buffer.txt` was one `--test-buffer` run away from. Deleted; the
  `Options` argument that fixed the cause is what makes them stale.

## Do not chase

- **The upstream debugging-stream diffs.** Characterised above, down to three source lines. Do not
  edit the comparison, do not relax `diff -b`, do not rewrite the upstream goldens. `-update`
  already refuses to touch that corpus.
- **`macrotim` line 9 and `strings`.** Missing features of AJB. Nothing to fix on this side.
- **`overflow`'s debugging stream.** Implementation dependent by the test's own admission; already
  in the `unstable` map with the reason recorded.
