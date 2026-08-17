# The L-map golden corpus

These L programs and the LOWL beside them are **original work**, written for this repository
against *Implementing software using the L language* (the L manual, kept at `.references/lmap.txt`).
They are not derived from, and not a transliteration of, `ml1aie.l` — the L source of ML/I, which is
copyright P.J. Brown and R.D. Eager and whose licence does not permit redistribution. The same rule
applies to the `.lwl` files: they are the LOWL **these** programs map into, not an extract of
`ml1aig.lwl`.

That distinction is why this directory exists. `ml1aie.l` and the published LOWL beside it are by far
the better test — 2,510 lines of real L, and the answer somebody else's L-map got from it — but
neither can be committed, so `TestMapML1AIE` skips on a fresh clone. This corpus is what gives a
fresh clone, and a machine with no network, real coverage of every statement.

## Rules for anything added here

- **Write the input yourself, from the manual.** Where a case needs the *shape* of something real —
  a chain with a labelled `ENDCH`, a `BACKSPACE` over a nested block — write your own statement with
  your own names.
- **Every case is a whole program**, `PRGSTART` to `PRGEND`, with its statements in SECTIONs that
  admit them. `sema` checks placement, and `TestGolden` refuses a case that does not resolve: a
  broken case tests the front end's diagnostics rather than the L-map.
- **Only `prelude.l` names an entry point.** The L-map takes the label `BEGIN` for the initialisation
  the MD-logic has to run first, so a case that used it would have that code in its golden instead of
  the statements it is named for. The others loop on `[START]`.
- **Read the golden before committing it.** `go test ./pkg/l/lmap -update` will write whatever the
  code currently does. That is a convenience for a case you have already reasoned about, not a way to
  find out what the answer is.

## Files

Per case `NAME`:

| file | what it holds |
|---|---|
| `NAME.l` | the input |
| `NAME.lwl` | the LOWL it maps into, up to where the MD-logic starts |
| `NAME.err` | the L-map's own diagnostics |

`.err` is optional, and **a missing one means "there must be none"** rather than "this is not
checked". `.lwl` is not optional: the harness refuses to create a golden that does not already exist,
so current behaviour cannot become the specification by accident. To add a case, create its `.lwl`
empty by hand and then run `-update`.

`mdlogic.lwl` is not a case. It is the other half of `prelude.l`'s output — the code chapter 7 of the
manual leaves to the implementor, which is the same in every program the L-map maps. Holding it once
is what keeps the twelve goldens about the twelve things they are named for.

## What each case is for

| case | what it holds |
|---|---|
| `exprs` | every shape an expression can take: a literal, a variable, `OF`, `AD`, `IND`, a block size, unary minus, and a chain of terms |
| `ifs` | both forms of `IF`, all five relations, `&` and `|` in both readings, character comparisons, and a labelled `END` |
| `backspace` | both forms, over a block and the block nested inside it, which is where the offsets come from |
| `chain` | `CHAIN FROM` with a body, with a labelled `ENDCH`, and with no body at all |
| `routines` | every call and every exit: a parameter of each type, an exit label, and the linkroutine |
| `moves` | `MOVE FROM` both ways, `MSTACK` on both stacks, `MUNSTACK`, and `BLOCK()` as an operand |
| `stack` | `STACK`, `UNSTACK`, `SCALE`, several targets in one `SET`, a target that is an address, and `SETSW` with both operators |
| `branch` | `GO TO`, `TEST` and `CHARMATCH` |
| `data` | a chain with every kind of item in it — a number, an `OF`, a length and text, a named character, and a link that points backwards |
| `opmac` | an operation macro that is global, one that is local, and one whose name is two atoms |
| `prefixes` | the three statement prefixes of appendix A, and the `INVALS` SECTION `/-IN-/` belongs to |
| `prelude` | a whole program: the one case that names an entry point, declares everything the MD-logic calls, and is therefore the one `TestAssembles` runs the assembler over |

## The one that matters most

`TestAssembles` is worth more than any of the goldens. A golden says the output is what it was last
time; it does not say the output is LOWL. The assembler works out where every word lands and refuses
an `RL` whose distance disagrees with the layout it made, an `EXIT` numbered past its subroutine's
exits, an operand of the wrong shape, and every name nothing defines — which is the complete list of
mistakes a code emitter makes.
