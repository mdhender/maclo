# The local golden corpus

These ML/I inputs and their golden files are **original work**, written for this repository
against the published ML/I documentation. They are not derived from, and not a transliteration of,
the test suite distributed on <http://www.ml1.org.uk/>.

That distinction is the reason this directory exists. The upstream suite is the better
conformance oracle, but its licence forbids redistributing a machine readable copy, so it cannot
live in the repository and every test that needs it skips on a fresh clone. This corpus is what
gives a fresh clone, and a machine with no network, real end-to-end coverage.

## Rules for anything added here

- **Write the input yourself, from the manual.** Work from `.references/ml1user.html` and
  `.references/ml1appaa.html`. Do not open the upstream `.ml1` files while writing a case. Macro
  semantics are not copyrightable; a transliteration would be.
- **Produce the golden with the reference implementation, then read it.** The oracle is
  **ML/I on Apple (Intel) under macOS, implementation version 4.13, ML/I version CKQ**, by Bob
  Eager, from <https://www.ml1.org.uk/impl-ac.html>, kept at `.downloads/ml1`. Confirm you have
  that exact build before generating anything — it should print
  `ml1: macOS version 4.13 (CKQ)` — because other ports of ML/I do not all agree byte for byte:

  ```sh
  .downloads/ml1 -v </dev/null
  .downloads/ml1 pkg/ml1/testdata/local/NAME.ml1 -o pkg/ml1/testdata/local/NAME.out -d /tmp/NAME.err
  ```

  It is an x86-64 binary, so on Apple Silicon it runs under Rosetta 2.

  This is safe with respect to the licence in a way that copying an upstream test is not: the input
  is ours, so the output is our own text passing through their processor. Nothing of theirs ends up
  in the file. **Read what comes back before committing it.** More than one case here first
  produced confident, wrong output because the author had misread a construct; the reference caught
  it, but only because someone looked.

- **Keep `NAME.err` out of the repository.** A clean run writes nothing to the debugging stream,
  because `S18` starts at zero and so no "At end of process" line is produced. The harness reads a
  missing `.err` as "this must be empty". Error-path wording is upstream's expression, so it is
  asserted in ordinary Go tests against *our* message text, never captured as a golden here — see
  `../../debug_test.go`, which covers `MCNOTE`, warning markers, an aborted construction and the
  `S18` report that way, and `../../storage_test.go`.
- **`-update` refreshes, it does not create.** It exists for a reviewed behaviour change. A golden
  that changes without an intended change is a bug report.
- **Whitespace is significant.** This corpus is compared **byte for byte**, unlike the upstream
  one, which has to be compared the way `runtest.sh` does, with `diff -b`. Trailing spaces, tabs,
  and the presence or absence of a final newline are all part of the expectation. The
  `.editorconfig` here disables trailing-whitespace trimming; check `cat -e` after editing,
  because most tools will quietly strip what these tests are checking for.
- **Avoid the operation macro names in text meant to pass through.** `MCDEF`, `MCSET`, `MCSKIP`
  and the rest are recognised from the start of a process, so a case that expects verbatim output
  must not contain them by accident.

## The cases

| case | covers |
|---|---|
| `passthrough` | text with no macros at all: blank lines, a line of only spaces, a leading tab, leading indentation, a trailing space, and runs of interior spaces. Every one of those is a difference `diff -b` would forgive and byte-exact comparison will not, which makes this the case that guards the comparator itself. |
| `define` | `MCDEF` with delimiter structures of one, two, and three arguments, and `%An.` insertion. Also the zero-argument form, `MCDEF ARRSIZE AS 6`: a name on its own is a whole delimiter structure, and with no arguments to delay there is nothing for literal brackets to do, so the replacement text is the rest of the line. |
| `nested` | three levels of macro, each calling the next from its replacement text. |
| `arith` | `MCSET` on permanent variables, and macro-time arithmetic including truncating division. |
| `skip` | `MCSKIP MT,<>`: literal text, nesting, and the empty skip `<>` used as a separator. |
| `layout` | the `SPACE` and `NL` layout keywords in a delimiter structure, and the fact that a delimiter is consumed. |
| `loop` | iteration by recursion, with `MCGO` for the base case, and `%%A1.-1.` forcing the argument to be evaluated rather than passed as text. Its prose states the label rule, which is easy to get wrong: a label is remembered as it is passed, so a jump to one already seen goes straight there, and only a label **not** yet seen provokes a forward search from the point of scan (§5.4.3, "Exact description of a 'goto'"). Both directions work inside a replacement text. What cannot jump backwards is the **source text**, which is why a loop has to live in a macro at all — not, as this case used to claim, because the search runs forward only. The case shows both directions: two macros recurse, and `COUNTUP` loops by jumping back to a label it has already passed, which is also the corpus's only use of `UNLESS`. Its counter has to be a permanent variable, because an argument does not change between iterations. Note the newline before the closing `>` of `COUNTUP`: the trailing `MCGO` needs one for its `NL` delimiter, and without it the case fails with "Delimiter (NL) of macro MCGO ... not found". |
| `atoms` | `S6`, the pseudo alphanumeric character. Setting it makes an underscore part of the atom around it, which changes both what a name matches and what a name may be; clearing it changes them back. |
| `listing` | `S20`, the listing control: no listing, a numbered one, a plain one, and the listing switched off and on again so that the numbers skip. This is the only case with a `.lst` golden. |
| `sysfun` | `MCLENG` and `MCSUB`, including the offsets from the end that a non-positive argument asks for, the ranges that give nothing, and the fact that the result is not evaluated again. |
| `inserts` | the insert flags — `A`, `B`, `D`, `WA`, and the null one — so the difference between evaluating an argument and inserting what was written is pinned. |
| `options` | `OPT`/`OR`/`ALL` alternatives, both the two-form kind and a delimiter that is its own successor, with `T1` saying which was taken. |
| `tempvars` | `T3`, the depth of nesting, and `T1`, the argument count, with an inner call proving it cannot disturb an outer one's set. |
| `stops` | `MCSTOP`, and the name clash that resolves in favour of the delimiter, which is the only way to show a stop marker without provoking an error. |
| `skipopts` | the `M`, `T` and `D` letters of `MCSKIP`: which of the text and the delimiters a skip keeps, and what a skip with neither leaves behind. |
| `alter` | `MCALTER` on a secondary delimiter of an operation macro and on a keyword, each changed and then changed back. |
| `pvars` | `MCPVAR`, and a variable named by an expression, which is how a subscripted array is written. |
| `arrays` | the composition `pvars` does not reach: a **permanent** variable subscripted by a **temporary** one, `PT1` and `PT2`. That pairing is what makes a run of permanent variables an array with a moving index, because the array is global and outlives the call while the index belongs to the call. The case fills, reverses in place with two subscripts at once, and walks the array from inside another walk of the same array — the last of which is the whole point, and is what an index kept in a permanent variable would fail. Note that `T1` starts as the argument count and is reassigned here, which §2.6.2 says is allowed. |
| `jumps` | the numeric conditions of `MCGO` together — `GR`, `GE` and `EN` at the same boundary, so that the only difference between them, what happens at equality, is visible in one place. There is no `LE` or `LT`; a test that wants one swaps its arguments. Also the label as a macro expression: `MCGO LT1` is a jump table indexed by a variable, and `MCGO L T1 + 10` gives the table a base. |
| `straight` | matched versus **straight** skips (§2.7.1). `skipopts` uses `MCSKIP T,` in passing; this shows what the missing `M` *means*: a straight skip does not recognise its own name inside itself, so it ends at the first closing delimiter. The third example is the subtle one — a straight skip met while scanning a matched one is matched first, so a `>` it swallows does not close the outer skip. |
| `startline` | `S1` and the `SL` keyword: the invisible character at the front of every input line that lets a newline be a macro name. |
| `exclusive` | the `N0` node flag, a closing delimiter left outside the construction it closes so that one newline can close two nested calls. Its golden has **no final newline**, which is the point. |
| `classes` | `MCGO ... BC` against the `N`, `L` and `I` classes, and the difference between `EN` and `=`. |
| `environ` | `MCNODEF` against a local and a global definition, the newline it does not consume, and redefining a name that has been deleted. |
| `streams` | `S10`, the input stream. Switching away from a stream and coming back to where it had reached, the end of a stream that is not the revert stream sending input back to the revert stream unasked, a stream already spent giving nothing, and `S10=102` — a hundred plus the number — rewinding a stream so it can be read a second time. The environment belongs to the process, so a macro defined in one stream works in the other. This is the only case with a second input file; see below. |

Traps worth knowing before adding a case, all of which caught the author of these:

- **Never write an `MC` name in prose.** The operation macros are recognised from the start of a
  process, so a sentence mentioning `MCGO` becomes a call to it.
- **The comparison operators are `GE GR EN BC =`.** There is no `LE` or `LT`.
- **A structure representation is evaluated, so a second pass over a definition is not idempotent.**
  §5.2.4 gives `MCDEF`'s order of evaluation as {arg A}, {arg C}, {arg B} — the name comes *last*,
  and it is evaluated like any other argument. So a file containing `MCDEF GREET AS <hello>`, read
  a second time after a rewind, expands `GREET` while working out what to define: the structure
  becomes `hello`, and what gets defined is a macro named `hello`. The `streams` case was written
  with the definition in the rewound stream and hit exactly this, reporting a missing delimiter for
  a macro named after a word in the replacement text. Put the definitions in the stream that is not
  rewound, or wrap the name in literal brackets to stop it being evaluated.

## A case with more than one input stream

A case may have extra input streams beside it, named `NAME.2.ml1` through `NAME.5.ml1` — five is
all ML/I accepts (AA.2). The harness feeds them in order after the case's own file, so `NAME.ml1`
is stream 1 and `NAME.2.ml1` is stream 2, which is where `MCSET S10=2` will look. They are not
cases themselves and need no golden of their own; `casesIn` skips them by name.

The numbering only works out because this corpus has no prelude. The upstream corpus has one, which
makes its case file stream 2, so the harness refuses the combination rather than misnumber it
quietly. Generate the golden by naming both files on the command line, in the same order:

```sh
.downloads/ml1 pkg/ml1/testdata/local/NAME.ml1 pkg/ml1/testdata/local/NAME.2.ml1 \
    -o pkg/ml1/testdata/local/NAME.out -d /tmp/NAME.err
```

Note that the oracle wants its workspace flag attached, `-w20000` and not `-w 20000`; spelt with a
space it reports `bad value for 'w' flag` and exits without writing anything.

A case is `NAME.ml1` with a `NAME.out` beside it. `NAME.err` and `NAME.lst` are optional: when one
is absent that stream is required to be **empty**, which is the right expectation for a clean run
because S18 and S20 both start at zero, so neither an "At end of process" line nor a listing is
ever written. Generate a `.lst` the same way as a `.out`, with `-l` alongside `-o`:

```sh
.downloads/ml1 pkg/ml1/testdata/local/NAME.ml1 -o pkg/ml1/testdata/local/NAME.out \
    -l pkg/ml1/testdata/local/NAME.lst -d /tmp/NAME.err
```
