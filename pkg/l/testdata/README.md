# The L golden corpus

These L programs and their golden files are **original work**, written for this repository
against *Implementing software using the L language* (the L manual, kept at
`.references/lmap.txt`). They are not derived from, and not a transliteration of, `ml1aie.l` —
the L source of ML/I, which is copyright P.J. Brown and R.D. Eager and whose licence does not
permit redistribution.

That distinction is why this directory exists. `ml1aie.l` is by far the better test: 2,510 lines
of real L written by someone who was not writing this parser. But it cannot be committed, so
`TestML1AIE` skips on a fresh clone. This corpus is what gives a fresh clone, and a machine with
no network, real coverage of every trap.

## Rules for anything added here

- **Write the input yourself, from the manual.** Do not copy lines out of `ml1aie.l`. Where a case
  needs the *shape* of something real — a `CHARMATCH` with a `'/'` arm, a labelled `ENDCH` — write
  your own statement with your own names.
- **Every case is a whole program**, `PRGSTART` to `PRGEND`, with its statements in SECTIONs that
  admit them. `sema` checks placement, so a fragment would report errors that are about the
  fragment rather than about the case.
- **The happy cases produce no diagnostics at all**, warnings included. That is not decoration: it
  means every variable is used and every label is branched to, which is what exercises the
  resolver rather than just the parser.
- **Read the golden before committing it.** `go test ./pkg/l -update` will write whatever the code
  currently does. That is a convenience for a case you have already reasoned about, not a way to
  find out what the answer is.

## Files

Per case `NAME`:

| file | what it holds |
|---|---|
| `NAME.l` | the input |
| `NAME.lst` | the statement listing, indented, with the source line in a gutter |
| `NAME.sym` | the symbol table |
| `NAME.err` | the diagnostics, warnings included |

`.sym` and `.err` are optional, and **a missing one means "there must be none"** rather than "this
is not checked". `.lst` is not optional: the harness refuses to create a golden that does not
already exist, so current behaviour cannot become the specification by accident. To add a case,
create its `.lst` empty by hand and then run `-update`.

## What each case is for

| case | the trap it holds |
|---|---|
| `layout` | all ten SECTIONs, and the two whose names break the identifier length rule |
| `vars` | every `DEC` form, `EQUATE`, a nested `BLOCKDEC`, and the `…SZ` constants a block derives |
| `comments` | the three comment forms; a comment containing a `/`; prefixes before *and* after a label; a prefix after `THEN` |
| `exprs` | every `OF` shape, nested `IND`, `IND(AD(X)PT)NM`, a leading unary minus, division |
| `ifs` | both forms, all five comparisons, three-relation `&` and `|` chains, a one-line IF inside a block one |
| `routines` | `SUBROUTINE` with and without a parameter and an exit, all three `CALL` forms, `LINKROUTINE` closing with `ENDSUB` |
| `chain` | a `CHAIN FROM` whose `ENDCH` carries a label that is branched to |
| `moves` | all three block moves, `BLOCK( )` on either side, both stacks |
| `branch` | `TEST` with ten targets; `CHARMATCH` with `'/'`, `'*'` and `'$'` arms; `STACK` with tight and spaced type tags |
| `io` | every `PRTEXT` shape, including significant trailing spaces, and `PRTEXT[SKIP]` in a file that also defines the label `[SKIP]` |
| `data` | `DC` with an argument of every kind, `[LAYCHN] LAYCHAIN`, `HETABLES`, `OPMAC` in both forms |
| `err_*` | one class of diagnostic each; `err_lexical` also proves recovery is per statement |

## Two cases worth understanding before you change them

`io.l` contains both `PRTEXT[SKIP]` and `[SKIP] SETSW …`. The bytes of the two brackets are
identical and only the word before them differs, which is the one lexical decision the scanner
makes. If that rule ever breaks, this is the case that says so.

`branch.l` writes `STACK IDPT (PT) …` with a space and `STACK FLAGSW(SW) 3(NM) …` without one.
Layout is insignificant in L, so both must parse the same way — which is why neither the scanner
nor the cst folds `NAME(…)` into a single node.
