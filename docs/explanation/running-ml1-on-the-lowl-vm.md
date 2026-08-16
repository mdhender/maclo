# Running ML/I on the LOWL VM

There are two ways to port ML/I. One is to implement L, the language ML/I is written in, and
translate the logic. The other is to implement LOWL, the low level language ML/I is distributed as,
and run the distributed source unchanged. This repository takes the second route: `pkg/lowl` is a
scanner, cst, ast, assembler and virtual machine for LOWL, and the intent is for `pkg/ml1` to be a
front end with a switch selecting a LOWL backend or, later, an L one.

This is the background you need before touching either half. For the sources themselves see
<http://www.ml1.org.uk/>; none of them can be committed here.

## The sources, and which version is which

Three separate things carry version letters, and they are easy to confuse.

| What | Version | Where it is | Notes |
|---|---|---|---|
| LOWL source for ML/I | AJB | `.downloads/lowlml1/ml1ajb.lwl` | the current one on ml1.org.uk |
| LOWL source for ML/I | AIH | `.references/lowlml1aih.tar.gz` | an older download, kept for comparison |
| LOWL source for LOWLTEST | L4A | `.downloads/lowltest/ltestl4a.lwl` | the LOWL conformance test |
| L source for ML/I | AIE | `.downloads/lml1/ml1aie.l` | for the L backend, unused so far |
| the reference implementation | CKQ | `.downloads/ml1` | macOS 4.13, produced every golden file |

Everything in `.downloads/` and `.references/` is gitignored on purpose: the licence forbids
redistributing machine readable copies. Do not commit any of it, or excerpts of it.

The version gap matters. Every golden file came from CKQ, and the newest LOWL source available is
AJB, so a golden mismatch may mean the port is wrong *or* that the source predates the feature.
The local corpus in `pkg/ml1/testdata/local` is entirely within reach of these sources; the upstream
suite has one keyword gap (`MCCVAR`, in `strings.ml1`) and its `.err` files, which carry error
wording and the end of process report, are the unbounded risk. Drive against the local corpus.

## LOWL is a kernel plus extensions

The LOWL kernel manual does not describe the whole language ML/I is written in. LOWL is "a kernel of
features plus some extensions specially tailored to each piece of software", and ML/I's extensions
are in a separate document:

- kernel: *Implementing software using the LOWL language* (`htmldoc/lowlmap.html`)
- **ML/I extensions: *… Supplement 3: ML/I* (`htmldoc/lowlml1.html`)**
- the L language: *the L manual* (`.references/lmap.html`)

Seven statements are extensions rather than kernel, and looking for them in the kernel manual is a
dead end: `HASH`, `THASH`, `WTHS`, `RL`, `LINKR`, `LINKB` and `ORL`. Supplement 3 also specifies the
machine dependent subroutines below. `pkg/lowl/README.md` marks the extension rows `[ML/I]`.

## The host boundary

The ML/I source calls seven subroutines that it never declares. They are the entire interface
between the LOWL logic and the outside world. The assembler recognizes them by name at the `GOSUB`
and turns each into a single instruction, so there is no return address to pop: the word after the
instruction is already the caller's jump table, and a subroutine with two exits does what `EXIT`
does and sets `Registers.JumpValue` for the `GOTBL` words to compare against. Exit numbers are one
based.

| Subroutine | What it must do | Where it lives |
|---|---|---|
| `MDREAD` | read one character into C. Two exits: source exhausted, or character read. Owns the input options, stream switching and newline conversion | `vm.Host` |
| `MDOUCH` | write C to the results stream, under the output options and the `S21` mask | `vm.Host` |
| `MDERCH` | as `MDOUCH`, but to the messages stream | `vm.Host` |
| `MDCONV` | convert `MEVAL` to decimal text; set `IDPT` to the start and `IDLEN` to the length | `vm/md.go` |
| `MDFIND` | hash the atom described by `IDPT`/`SPT`/`IDLEN` and set `HTABPT` to its chain head | `vm/md.go` |
| `MDOP` | multiply or divide `OP1` by `MEVAL`, under `OPSW`. Two exits, one for divide by zero | `vm/md.go` |
| `MDQUIT` | end the process | `vm/step.go`, as `ErrQuit` |

Only three of them are policy. `MDCONV`, `MDFIND` and `MDOP` never reach outside the machine: they
read and write the program's own variables, so the VM implements them itself and finds those
variables through `vm.Symbols`, the name-to-address map the assembler hands over. The other three
are the `vm.Host` interface, which is what keeps `pkg/lowl` free of any dependency on `pkg/ml1`.
They map almost one to one onto the `Job` model documented in `pkg/ml1/ml1.go` — numbered input
streams selected by `S10` reverting to `S23`, an output bit mask in `S21`, a debugging stream with
the `S12` quota, `S5` as the error count.

Three details are easy to get wrong and expensive to find:

- **`MESS` writes to the messages stream, not the results stream.** The LOWL manual defines it that
  way, and it is how the end of process report reaches the debugging file. It goes through
  `Host.WriteMessage` a character at a time rather than straight to a writer, because that stream is
  metered: `S12` holds a quota of lines and the host is what counts them.
- **`MDERCH` writes the character in C as it stands.** The dollar sign only means a newline inside
  the text of a `MESS` statement.
- **`MDOP` divides the way chapter 2 of the user's manual asks**, giving the greatest integer that
  does not exceed the exact answer. Go's own division truncates towards zero, so it answers -1 to
  the manual's `-5/4` and `5/-4`, which are both -2.

`MDCONV` hands its answer back as a pointer, so the digits have to outlive the call. `vm.New`
reserves `MaxNumberText` words for them below the program, where neither the assembled logic nor
the workspace can reach.

## What the host must set up before BEGIN

The first instruction of ML/I is `LAV FFPT`, so the storage has to exist before it runs. Two things
are needed and neither is optional.

**The workspace.** `FFPT` points at the first free word and `LFPT` at the last; the forwards stack
grows up from one and the backwards stack down from the other. Both live *in core*, past the end of
the assembled program — `FSTK`, `BSTK` and `CFSTK` reach them through those two variables, so they
cannot be a separate array. `vm.SetWorkspace` does this, and `vm.Run` leaves an existing layout
alone. A workspace starting at address zero silently overwrites the program: ML/I carves its
permanent variables out of the space `FFPT` points at, so the damage shows up much later and
somewhere else entirely.

**The S-variables.** `SVARPT` is the one variable ML/I reads and never writes; of the 91 it
declares, the only others never written by the source are `LINKPT` and `PARNM`, and the assembler
emits the code that writes those. The block is stored in reverse order, S1 last, with a count above
it that `SVARPT` points at:

```
Core[SVARPT]            the count of S-variables
Core[SVARPT - n*LNM]    Sn
```

`vm.SetSystemVariables` builds it and returns the first address after the block, so a host lays its
storage out front to back — S-variables first, then `SetWorkspace` behind them. `S2` is the line
count and `S5` the error count, which is how the layout was confirmed. Without this, the first
access computes a negative address.

The values are the host's, because everything from `S10` up is implementation dependent. There are
24 of them; `S1` to `S9` are ML/I's own and start at zero except `S6`, which is -1. The reference
implementation's defaults were read straight out of the oracle, with an input that inserts each
variable in turn, and that is worth doing again rather than reasoning about: `S2` looks as though it
should start at 1, because it is the number of the line being read and the first line is line 1, and
it does not. Only one other is not obvious: `S24` is a bit per output stream saying that stream is
at the start of a line, and every stream is, so it starts at 15 rather than 1.

That reading is not infallible, and `S19` is where it failed: inserting a variable is itself output,
so a variable the act of output changes reports the value it has *after* the insertion rather than
before. See the listing below.

## S6 belongs to the machine, not to the program

`S6` holds the code of a character that is neither a letter nor a digit but is to be counted as part
of the atom around it — the underscore of `CURRENT_POSITION`, in chapter 2's example. Looking for it
in the LOWL source is a dead end: `SVARPT` is read in five places and none of them is S6, and
Supplement 3 does not mention the variable at all.

That is because ML/I splits text into atoms with one statement, `GOPC`, and nothing else. The kernel
manual defines `GOPC` as "branch if the character in C is a punctuation character, i.e. not a letter
or a digit", so the only place the extra character can be honoured is inside the implementation of
`GOPC`. `vm.ispunct` is therefore a method: it reads S6 out of the S-variable block through
`Registers.SVARPT`, which `SetSystemVariables` fills in, and a program with no S-variables — LOWLTEST
— gets the kernel's plain definition. The value is read on every call rather than cached, because a
macro can assign to S6 between any two characters of the source text.

Nothing fails loudly when this is missing. Every name still matches, because a name written with an
underscore in it is three atoms on both sides of the comparison; what changes is that the user cannot
turn that off, so `MCDEF CURRENT` quietly rewrites the middle of `CURRENT_POSITION`.

## The listing, and where an output line begins

The listing is entirely the host's. `MDOUCH` is handed one character at a time and the LOWL source
knows nothing about `S20`; appendix AA gives it three values — zero for no listing, two for one with
the number of each line in front of it, one for one without — and leaves everything else unsaid.
The rest was measured against the oracle, and none of it is obvious:

- **The listing is a copy of the output text, not of the source.**
- **It is taken before `S21` is applied.** A line the output mask sends to no file at all is still
  listed, and still counted.
- **`S20` is read for every character, not once a line.** A macro can switch the listing off in the
  middle of a line and the rest of that line is simply absent. Nothing marks the gap.
- **The number is `%5d.` followed by three spaces**, and it is not truncated when it outgrows the
  field.
- **The numbers are of output lines, not of listed ones.** A listing switched off for a while
  resumes at the number the count has reached, which is the only sign that anything is missing.

The number comes from `S19`, and `S19` is where the surprise is. It is the number of the line being
written rather than the count of the lines finished: it **starts at zero and is stepped by the first
character of a line**, not by the newline that ends the one before. A macro can see the difference,
because between two lines it reads the number of the line just finished; so can anyone who sets it,
because `MCSET S19=9` makes the next line the tenth. The empty line is a line like any other — the
newline is its own first character.

This is also why `systemVariables` does not list `S19`. Reading the reference implementation's
defaults with an input that inserts each variable in turn gives one, and one is wrong: inserting
`%S19.` is itself output, so it steps the count before it can report it. Every method of
measurement is part of the measurement.

## The hash table

`HASH` and `THASH` are two halves of one structure. Each built-in macro name gets one `HASH` item
holding its chain link; the single `THASH` after them reserves one head per chain. The assembler
threads the chains once the source has been read, because a link points at an entry it has not seen
yet. A chain head holds the address of the first entry's link word, each link word holds the address
of the next, and zero ends the chain; the entry body starts one word past its link.

The bucket calculation is `vm.HashName`, and `MDFIND` must call the same function. Nothing outside
those two depends on which bucket a name lands in, but if they ever disagree, a name that is present
will not be found.

## The end of process report is not gated by S18

Appendix AA says the "At end of process: N lines, M calls" line is written only when bit 2¹ of
`S18` is set, and the list of defined constructions only when bit 2⁰ is. **The LOWL source has no
such test.** Its finalisation code, at `LOHALT`, writes both unconditionally and then calls
`MDQUIT`; the MI-logic never reads `S18` at all, and the only S-variables it touches are `S2`, `S3`,
`S4` and `S5`.

This is not a mistake, it is the L/LOWL split. In the L source, the end of source text goes to
`MDHALT`, which is the host's, and the host writes the report it wants. The LOWL translation folded
a default `MDHALT` into the MI-logic as `LOHALT`. `ml1ajb.lwl` was last updated in 1986; the
oracle's `S18` gating belongs to the much later C implementation derived from L.

So the report has to be suppressed on this side of the boundary, or every golden `.err` in the local
corpus — all of which are empty, because `S18` starts at zero — will fail. The host knows where
`LOHALT` and `PRENV` are, because `vm.Symbols` has them, and each is reached exactly once, at the
end: the statistics start at `LOHALT` and the list of constructions starts at `PRENV`. That is why
`pkg/ml1` drives the machine in a loop of its own instead of calling `vm.Run` — the loop watches for
those two addresses and tells the host which part of the report is being written, and the host
applies the two bits of `S18` to it. The upstream suite needs both halves: its prelude sets `S18` to
2, which asks for the statistics and not the constructions.

## Three things that were wrong for a long time

All three were found by running real ML/I, and none could have been found any other way: the machine
assembled cleanly and LOWLTEST passed with every one of them in place. The third also shows what the
first two do not — that a mapping macro's last statement is as much a part of it as its first.

**`RL` stores a distance, not an address.** `RL table label,OF(expr)` defines a word holding the
offset *from itself* to the label — the LOWL supplement's own example maps it as `PIG - .` — and the
`OF` expression is that same distance written in `LNM` and `LCH` so that an assembler without a
location counter can produce it. ML/I walks its tables by adding a word to the address it read that
word from, so an absolute address there is not a wrong number in any visible way: everything
assembles, most of the processor works, and what eventually appears is a delimiter printed as the
middle of some other string. The assembler now stores the distance and checks it against the
expression the source gave, which turns a table that has been laid out into a different shape from
the one it was written in into an error at assembly time.

**Running out of storage is a branch, not a fault.** The kernel manual maps `FSTK`, `BSTK` and
`CFSTK` as ending in `GOGE ERLSO`, and says of that label that it "is present in every MI-logic".
The machine used to treat the overflow as an error of its own and stop, which threw away everything
the program would have said about it: `ERLSO` prints the diagnostic, counts the error in `S5`,
prints the context that says where the storage went, and then ends through the ordinary
finalisation at `LOHALT`. A process that ran out of workspace therefore died with an **empty
debugging stream**, which reads exactly like one that finished cleanly. `vm.stackOverflow` now
branches to `ERLSO` when the program has one, and only reports and stops when it has not — LOWLTEST
being the program that has not.

**`CSS` clears the subroutine stack; it does not pop one link.** The kernel manual says a `CSS`
appears after each label in the main logic that is branched to from inside a subroutine, and such a
branch can come from any depth, so the whole stack goes. An empty stack is not an error either,
because the same labels are also reached by falling into them.

## Where this stands

Done:

- All of `ml1ajb.lwl` and `ml1aih.lwl` assemble, with no undefined symbols.
- **LOWLTEST L4A assembles and runs to completion with no failures.** The VM is conformant: GO
  forward, backward and long, arithmetic and conditional GOs, GOSUB, EXIT 1 to 4, GOADD, FSTK, BSTK,
  GOPC and character output all pass. Its output is on the messages stream, so `lasm` leaves it in
  `vm_stdmsg.txt`. `TestLOWLTEST` in `pkg/lowl/vm` now runs it under `go test`, skipping when the
  program has not been fetched. It is the only end-to-end test of the kernel: `op_test.go` checks
  opcodes one at a time against expectations built by hand, and this is a real program exercising
  them against each other.
- All seven MD subroutines run. Three are the `vm.Host` interface and three are in `vm/md.go`;
  `MDQUIT` was already `ErrQuit`.
- `ml1.Run` assembles the LOWL source in memory and runs it against a `Job`. `pkg/ml1/host.go` is
  the MD-logic: the `S10`/`S23` stream switching and rewinds, the `S16` translation, the `S21` mask
  and `S24` line state, the `S12` quota and the `DebugWidth` wrap.
- **The whole local golden corpus passes, byte for byte.**
- `S6` is honoured, in `GOPC`. The `atoms` case of the local corpus covers it, and the upstream `s`
  case now matches its golden output stream.
- **`Job.Listing` and `S20` produce a listing**, and `S19` is stepped where the oracle steps it. The
  `listing` case of the local corpus is compared byte for byte, as is every case's empty one.
- **Every opcode has a test.** The eight `not tested` markers are gone, and the two they were
  hiding are fixed: `CFSTK` stepped `FFPT` by `LNM` where the manual says `LCH`, and `EXIT` indexed
  off the end of an empty return stack instead of reporting it.
- **`cmd/fetchtestdata` fetches the LOWL source**, and LOWLTEST with `-corpus all`, so a fresh
  clone needs no hand downloads to run everything but the oracle.
- **The local corpus covers 21 cases**, from `MCLENG` and the insert flags to startlines, exclusive
  delimiters and `MCALTER`. Widening it found the storage bug below, which is what it is for.
- **The four fatal conditions of AA.4.1 are reported in ML/I's shape.** The quota, an illegal `S10`,
  a rewind that fails and an output file that cannot be written used to put the Go error text on the
  debugging stream in one line, sentinel and all, with no error counted in `S5`. They now write the
  `Error(s)` prologue that §6.3 says `S5` counts and then hand control to `ER7A` for the context
  print-out and the finalisation, which is the same move `ERLSO` makes below and for the same
  reason: the context walks ML/I's own stack of environments, so only the program can produce it.
  `pkg/ml1/fatal_test.go` pins the shape, and each case was compared against the oracle first.

Not done:

- **The upstream corpus fails, and the reason is the version skew.** Running the whole T2A suite in
  `.downloads/ml1tests` against the live oracle, side by side, is the sharpest measurement of where
  the port stands: **all ten output streams are identical**, and four of the ten debugging streams
  are identical while six differ. The differences are only ever two things, and both are in the
  error print-outs:
  - **One blank line too many.** `MESS '$$$Error(s)$'` puts three newlines where the oracle puts
    two, and the `$$` that ends a context print-out puts two where the oracle puts one. A `$$` in
    the middle of a block agrees exactly. So it is not a rule about the stream; the CKQ text of
    those particular statements has one `$` fewer.
  - **Spaces around the arguments** in the "with arguments" display: an extra trailing space on the
    heading and an extra leading space on each argument.
  Two cases differ for reasons of their own, and both are the source being older than the golden
  rather than the port being wrong. `strings` uses `MCCVAR`, which this source does not have, so it
  produces errors until the `S12` quota runs out. `macrotim` uses the bitwise operators `&` and `|`
  of a macro expression, which chapter 2 of the sixth edition documents and `GETEXP` does not
  implement: it accepts `+ - * /` and rejects anything else as an illegal argument. Reading that
  routine is the whole of the diagnosis, and there is nothing to fix on this side of the boundary.

One note for whoever picks this up: every stage still writes its debug artifacts to the current
working directory, which `ml1.Run` must not do — it runs in process, from buffers. That is opt-in
plumbing on the caller's side (`cst.ParseBuffer` rather than `cst.Parse`, `assembler.Options{}` with
`Listings` unset, a nil `vm.Streams.Trace`), and opt-in plumbing is the kind that gets forgotten, so
two tests now hold it in place: `TestRunWritesNothing` runs the engine from an empty directory and
requires it to still be empty, and `TestDebugArtifactsAreDeclared` reads the source and requires
every file-creating call under `pkg` to be one of the six that belong to `lasm`. The second exists
because the first can only see a write on a path it exercises.

`TestGoldenUpstream` is now the only expected failure in `go test ./...`; anything else is a
regression.
