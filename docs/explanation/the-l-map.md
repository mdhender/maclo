# The L-map

ML/I is distributed twice. The `.lwl` files this repository runs are LOWL, a low level language with
one opcode per line; the `.l` file beside them is **L**, which is what the logic is actually written
in. The L manual's word for a translation from L into some object language is an *L-map*, and every
statement in the manual carries an **Action** clause saying what the object code has to do. Writing
one is how you port ML/I to a machine that has no LOWL implementation.

`pkg/l/lmap` is an L-map whose object language is LOWL. This page is why that is not circular, what
the manual specifies, what it leaves to the implementor, and how much of the answer can be checked
against somebody else's.

## Why LOWL and not a machine of its own

The obvious alternative was a virtual machine that walks the L tree directly, the way an interpreter
would. It was rejected for three reasons, and only the first is about effort.

**There is an answer key.** `ml1aig.lwl` *is the output of the exact translation this code performs*,
from a source one release away from `ml1aie.l`, and it is already on disk. Every design question —
what a `BACKSPACE` offset is measured from, what the fields of an operation macro entry are, what
`CHAIN FROM` compiles into — has a published answer that can be read and diffed. A tree-walking
machine could use none of it.

**A tree walker would not be simpler.** L is address-based: `IND`, `AD`, `RL`, `MOVE FROM … LENG`,
`STACK … ON FSTACK`, `OF(2*LNM+LCH)`. It needs a word-addressed store, which `vm.Core` already is,
and its own machine-dependent logic, which `pkg/lowl/vm` already has. What it would save is the
assembler, and the assembler is the part that catches mistakes.

**L's control flow crosses the tree.** `GO TO MBEGIN` leaves the `INVALS` SECTION entirely.
`GO TO LULRET` jumps from inside a `CHAIN FROM` body into an `IF` block that has already been left.
A walker would have to flatten the tree to a label-indexed list before it could run at all — which
is an assembler, with no artifact to diff.

So the pipeline is:

```
ml1aie.l
  → pkg/l          scanner, cst, ast, sema
  → pkg/l/lmap     Map, then WriteLOWL
  → pkg/lowl       cst, ast, assembler
  → pkg/lowl/vm    the machine pkg/ml1 drives
```

and `ml1.Run` chooses between it and the LOWL route on one field of the `Job`.

## What the manual specifies, and how literally

The Action clauses come in two kinds, and the difference decides how much judgement each statement
needs.

Most are prose. `GO TO` is "branch to designated label"; `SET` is "evaluate the arithmetic expression
and assign its value to the variables and/or indirect address on the left of the equals sign". These
are the ones where the object language answers the question: LOWL has `GO`, and it has a load, an
add, a subtract and a store, so a `SET` is one load and one store per target.

The awkward ones are written **as other L statements**, which makes them mechanical rather than a
matter of taste. `BACKSPACE V GIVING W` is defined as `SET W = IND(DBUGPT + N)`, where *N* is the
number of units of storage occupied by the variables preceding *V* in the block called SDB. So the
L-map walks the `BLOCKDEC` — with `EDB` nested inside `SDB`, counted from the start of `SDB` — and
emits a base-register load and an offset. The published LOWL agrees on every one of them: `DBUGSW`
at 8, `SPT` at 4, `STOPPT` at 12, `INFFPT` at 14, `DBUGPT` at 3, `ARGPT` at 2, `STAKPT` at 1.

`MSTACK FROM` is the same kind of definition and the one worth reading twice, because it names two
subroutines of the program being mapped:

```
CALL DECLF(%A2.)NM
MOVE FROM %A1. TO LFPT LENG %A2.
```

That is deliberate rather than a leak. Growing a stack has to check for overflow, and what to do
when it overflows is the logic's business, not the machine's: `DECLF` and `BUMPFF` report it through
ML/I's own diagnostic. The same reasoning runs through chapter 7, which is why the MD-logic in
`prelude.go` calls `PRNUM` and `PRENV` and branches to `ERLIA` and `ERLOVF`. An L-map is written for
one program.

`CHARMATCH` and `MOVE FROM` are also written out, the second as a character-at-a-time loop with the
explicit note that an L-map "need not follow the exact descriptions … In fact the whole purpose of
the block moving statements is to allow efficient code specially tailored to the object machine to be
inserted". LOWL has `FMOVE` and `BMOVE`, so none of the three becomes a loop.

## The three decisions the manual leaves open

**Data type identity.** Section 3.1.1 says data types need only be told apart if they are stored
differently, and recommends that numbers and pointers be represented the same way, "as it obviates
the need to examine the data types when generating code for arithmetic expressions". This L-map goes
further and makes switches the same too, so the length macros `LPT` and `LSW` both become `LNM`.
That is not only a simplification: `pkg/lowl/assembler` seeds names for `LCH`, `LNM`, `LICH` and
`LHV` and for nothing else, so `OF(4*LPT)` would not resolve. The L source uses `LPT` 24 times and
`LSW` 11, and neither appears in the published LOWL either.

**One parameter name.** A LOWL `SUBR` can store its argument into `PARNM` and nowhere else. The
manual anticipates this — the three names "could in fact be equated to one another in an L-map where
types were not differentiated, as the logic of ML/I is such that there is never more than one
parameter in existence at any one time" — so `PARPT` and `PARSW` become `EQU`s.

**`THEN GO TO` as a special case.** Note (c) on the `IF` statement says "many L-maps will wish to
recognise THEN GO TO as a special case, in order to generate better code", and it is worth it: 133
of the 203 `IF`s in ML/I are that shape, and a condition that is already a branch needs no join label
at all. The other 70 invert the relation and branch over the guarded code. L has no "less than"
operator and LOWL does, which is exactly what the negation of `GE` needs, so nothing has to rewrite a
comparison in order to invert it.

## The MD-logic

Chapter 7 lists what L does not describe, and the answer to most of it is that this machine already
has it. `MDCONV`, `MDFIND` and `MDOP` are instructions of `pkg/lowl/vm`; `MDTEST` asks whether a
character can be part of an atom, which is what `GOPC` branches on, and the manual says in so many
words that it is highly desirable to replace it with in-line code.

What is left is in `prelude.go`, written from the manual as our own text:

| | |
|---|---|
| `LOSCHN`, `LOECHN` | what `CHAIN FROM` and `ENDCH` compile into: load the first link, and add the next |
| `LOERPR` | `MDERPR`, which every error message is printed through, and which shows the startline character as `(SL)` |
| `LONUM` | `MDNUM`, whose two exits separate "this was never a number" from "this is a number and it is wrong" |
| `LOREAD` | the `READ` statement: the character, and the line accounting and stop code around it |
| `LOOUTP` | the `OUTPUTID` statement |
| `LOGOBC` | `MDGOBC`, the `BC` delimiter of the `MCGO` macro |
| `LOHALT` | `MDHALT`, and `MDABRT` with it |
| the code at `BEGIN` | reserving the error block and the permanent variables, deriving the markers, and setting the limits on local definitions |

It is written from the manual rather than copied out of `ml1aig.lwl`. The licence on the LOWL and L
sources permits building them into a program and not redistributing them, so a tracked file
transcribed from the distribution would break it. `ml1aig.lwl` is what the prelude is *checked*
against, the same way the reference binary checks the golden corpus without being committed.

Four constants are also decisions rather than facts. `OPMK`, `LOCMK`, `STRMK` and the two insert
markers are tags in one field, and all that is required of them is that they differ; `TEXMAX` and
`HTMAX` are how much of a long piece of text an error message prints before it gives up. The values
here are the published ones, so that the two engines produce the same diagnostics.

Four more are not constants at all in this L-map. `WITHMK`, `WTHSMK`, `EXCLMK` and `SPCSMK` have to
be values no input character can have, and the only one that can be named is the marker the assembler
put in the tables for a composite operation macro name. So the initialisation code reads that one and
counts down from it, and L's four constants are variables here.

## How much of it is checked

**The tables are checked exactly.** The 246 words the data SECTIONs of `ml1aie.l` describe come out
identical to the published LOWL, `RL` distances included. That is the part with no room for opinion:
a table is walked by adding the word just read to the address it came from, so a distance that is off
by one is not a wrong number, it is a pointer into the middle of a string. `pkg/lowl/assembler`
checks every claimed distance against the layout it made, which makes this both an assertion and a
free self-check on the whole data section.

**The code is not, and should not be.** The published LOWL was written by a translator that merged
adjacent statements, reused the accumulator across them, and turned `SET V = V + 1` into a single
increment. This one emits each statement on its own. Both are correct L-maps and the outputs differ
on most lines, so a line-by-line diff would be measuring style. What is compared instead is the set
of subroutines — every one the L source declares appears in the published LOWL under the same name —
and, past that, behaviour.

**Behaviour is checked byte for byte.** `TestLBackendMatchesAIG` runs the whole local corpus through
the engine this produces and through the published AIG engine and requires the results stream and
the debugging stream to agree exactly, on all 25 cases. AIG is compared against rather than the
committed golden files because those were recorded against AJB, two versions later than the L
source: comparing to them would report the wording ML/I changed between releases as a fault of this
translation.

## The defect in the source

The L source as the archive ships it does not resolve. A `TEST` branches to a label whose declaration
is spelled with a letter too many, and the LOWL distribution settles which of the two is the typo: it
spells the label the way the branch does. `TestML1AIE` asserts that this is still the only undefined
name in the file, and there is deliberately no allowlist, because a mechanism for excusing corpus
errors is a mechanism for hiding regressions.

The back end refuses it. Reporting and carrying on is right for a listing and wrong for an engine: a
branch to a label nothing declares maps into a branch to a label nothing defines, and the assembler
would then report a fault of the generated text rather than of the source it came from. So mapping
the real source needs a corrected copy, which the archive does not contain and this repository cannot
ship. `go test ./pkg/l/lmap -run TestMapML1AIE -v` says how to make one, and the tests that need it
skip on the file rather than failing.
