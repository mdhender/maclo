# The upstream test suite, and why it is not in this repository

This port needs to be judged against something. ML/I has an obvious oracle: the test suite that
ships with the reference implementation, eleven cases with recorded output covering delimiter
structures, macro-time evaluation, skips, inserts, and error reporting. Running it is the most
direct evidence that the port is correct.

We cannot commit it.

## The constraint

The ML/I sources, tests, and documentation are copyright P.J. Brown and R.D. Eager. The licence
reads:

> Permission is granted to copy and/or modify this document for private use only. Machine readable
> versions must not be placed on public web sites or FTP sites, or otherwise made generally
> accessible in an electronic form.

A public git repository is exactly "otherwise made generally accessible in an electronic form", and
git never forgets, so committing the suite once would be committing it permanently.

This extends further than it first appears. The `.ml1` inputs are obviously covered. So are the
`.out` and `.err` files, because they are derived works of copyrighted inputs — generating them
ourselves by running our own processor would not launder them, since what makes them valuable is
precisely that they encode someone else's test program. That is why `testdata/`, `.downloads/`,
`.archive/`, `.notes/`, and `.references/` each carry a `.gitignore` containing `*`, and why
`cmd/fetchtestdata` refuses to write anywhere that is not covered by one.

## What is safe to commit

A **SHA-256 is not a copy.** It is thirty-two bytes from which nothing can be reconstructed. A file
name and a byte count are likewise facts about a work rather than the work. So
`cmd/fetchtestdata/manifest.json` records, for each archive, the URL, the archive digest, and the
name, size, and digest of every member, and that buys two things a bare download script would not:

- **Integrity.** Nothing is extracted until the archive matches, and nothing is accepted until every
  member matches. A truncated download or a half-populated directory is an error, not a mystery.
- **Drift detection.** If upstream re-rolls the suite, the fetch fails loudly and a person has to
  look at what changed. Without the digests, a change upstream would silently redefine what we
  consider correct.

## Why there are two corpora

Fetching solves the licence problem but creates a coverage problem: a fresh clone, a CI runner, or
anyone offline has no corpus, so every golden test skips and the suite proves nothing.

So there is a second corpus in `pkg/ml1/testdata/local/`, written from scratch against the
published manual and committed. The two are not redundant:

| | upstream | local |
|---|---|---|
| authority | the reference implementation's own recorded behaviour | our reading of the manual |
| availability | after an explicit fetch | always |
| comparison | `diff -b` | byte for byte |
| debug wrap | 72 columns | never |

Macro semantics are not copyrightable; a specific test program is. So the local corpus has to be
*independently written*, not transliterated — different macro names, different sample text,
different structure. That rule is restated in that directory's `README.md`, where someone about to
add a case will actually read it.

## Why the comparison differs between them

The upstream suite comes with its own harness, `runtest.sh`:

```sh
../ml1 -v sets18.ml1 $1.ml1 -o $1.tmp_out -d $1.tmp_err
diff -b $1.out $1.tmp_out
```

`diff -b` ignores the *amount* of whitespace: trailing blanks vanish, and any run of spaces or tabs
matches any other run. It does not ignore the *presence* of whitespace, so `foo` and ` foo` still
differ.

Conforming to that is an obligation rather than a preference. Those golden files were recorded from
a run that was judged by `diff -b`, and several of them genuinely carry trailing whitespace. Holding
them to a stricter standard than the people who produced them would mean failing on differences they
never claimed to control.

The cost is that `-b` hides a real class of bug: emit one space too many between two words, or strip
a trailing space, and the upstream corpus will not notice. That is exactly why the local corpus is
compared **byte for byte**. We write both sides of that comparison, so there is nothing to conform
to, and in a macro processor a stray space is a defect rather than a formatting quibble. The
`passthrough` case exists to carry precisely the whitespace `-b` would forgive.

### The slack is currently unused

Worth recording, because it bears on whether `-b` is needed at all: running the reference
implementation — **ML/I on Apple (Intel) under macOS, implementation version 4.13, ML/I version
CKQ** — against its own suite reproduces **all 22 golden files byte for byte**, not merely
`diff -b` equal. So `-b` is slack that build does not draw on. It is presumably there for ports
whose whitespace handling differs, which may well include this one before it is finished. That is a
statement about one port, though: ML/I is a family of implementations, and another may well need
the slack.

That leaves a judgement call. Holding upstream to byte equality today would pass, and would catch
more; but it would also mean failing on differences the suite never claimed to control, and the
`-b` in `runtest.sh` is the closest thing there is to a statement of what upstream considers
correct. We conform, and get the strictness from the local corpus instead.

The same reasoning drives `Job.DebugWidth`. The reference implementation hard-wraps its debugging
output at column 72, mid-word, and every upstream `.err` file is wrapped that way, so matching them
means wrapping identically — `diff -b` will not rescue a different wrap column. But 72 is an
artefact of a 1970s terminal, not something worth baking into our own expectations, so the local
corpus runs at `NeverWrap` and its golden files record the message text itself. One knob, two
settings, rather than a compromise that serves neither.

## Why one upstream case can never pass

`overflow` tests stack exhaustion through runaway recursion, and its own header says:

> Depth of nesting reached before overflow occurs will vary between implementations

Its `.err` records five nested `PIG` frames before storage ran out — a number that falls out of the
reference implementation's workspace accounting.

The reference reproduces it exactly, of course; it is the implementation the file was recorded from.
The exclusion is not about that build being unstable, it is about *ours*. Matching those five frames
would mean matching the reference's workspace accounting word for word, which is a far stronger
requirement than matching its output, and not one worth designing the storage model around. So the
`.out` is still compared and the `.err` comparison is skipped, with the reason logged rather than
hidden, so it reads as a known limitation instead of a mysterious exclusion. If the storage model
ever does line up, the exclusion can go.

## Why the tests skip instead of failing

Both corpora skip when `ml1.Run` reports `ErrNoEngineSource`. The engine is the LOWL source of ML/I
itself, which cannot be committed here either, so a clone that has downloaded nothing has no
processor to test as well as no corpus. `cmd/fetchtestdata` fetches both, under the same digests
and into the same kind of ignored directory, so one command is the whole answer.

Failing loudly was the alternative. Adding eleven failures for a machine that simply has not
downloaded anything would say nothing about the port, and it would compete with the one failure
that does mean something — `TestGoldenUpstream` on the version skew.

The important property is that the skip **expires by itself**. It is keyed on the engine's own
behaviour rather than on an environment variable or a build tag, so the day the source is in place
the tests start running, with nobody needing to remember to switch them on. A skip that requires a
human to remember it is a skip that becomes permanent.

## What they say today

The upstream cases run, and they fail. Their output streams match apart from `macrotim`, which uses
the `&` and `|` of a macro expression that this source's `GETEXP` does not implement;
their debugging streams do not, because the engine is the 1986 LOWL source and the golden files came
from the 2006 C implementation, which lays its error print-outs out differently. Treat this corpus
as diagnostic until a source of the same vintage as the golden files is available, and read
[Running ML/I on the LOWL VM](running-ml1-on-the-lowl-vm.md) before concluding that a difference is
a bug in the port.
