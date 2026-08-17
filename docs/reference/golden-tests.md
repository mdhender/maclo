# Golden tests

Reference for the golden file harness in `pkg/ml1`. For how to get the upstream corpus, see
[the how-to](../how-to/fetch-the-upstream-sources.md); for the reasoning, see
[the explanation](../explanation/upstream-test-suite.md).

## The oracle

Golden files for the local corpus are produced by the reference implementation, and the upstream
golden files were recorded from it. Exactly which build matters, because "ML/I" is a family of
ports and they do not all agree byte for byte.

| | |
|---|---|
| implementation | ML/I on **Apple (Intel) under macOS** |
| implementation version | **4.13** |
| ML/I version | **CKQ** |
| author | Bob Eager |
| built with | clang, C-map method |
| source | <https://www.ml1.org.uk/impl-ac.html> |
| self-report | `ml1: macOS version 4.13 (CKQ)` |
| kept at | `.downloads/ml1` (gitignored; not redistributable) |
| sha256 | `4ab419fafe8ecdcfd26c9701f7d15f74bb0a00deca4579ee8009c95601843fae` |

It is an **x86-64** Mach-O binary, so on Apple Silicon it runs under Rosetta 2. Check you have the
same one before trusting a golden file you generated:

```sh
.downloads/ml1 -v </dev/null      # ml1: macOS version 4.13 (CKQ)
shasum -a 256 .downloads/ml1
```

This build reproduces all 22 upstream golden files byte for byte, which is what qualifies it as the
oracle. A different port, or a later version, may not — if you replace it, re-verify against the
upstream suite before regenerating anything.

`TestOracleIdentity` in `pkg/ml1` enforces both the digest and the version string whenever the
binary is present, and skips when it is not. So this table is checked rather than merely asserted:
regenerating a golden file with a different ML/I fails the build instead of quietly changing what
the port is measured against.

## Corpora

| | `local` | `upstream` |
|---|---|---|
| test | `TestGoldenLocal` | `TestGoldenUpstream` |
| directory | `pkg/ml1/testdata/local` | `testdata/upstream/tests-ac` |
| in git | yes | no, fetched |
| prelude | none | `sets18.ml1`, as input stream 1 |
| comparison | byte for byte | `diff -b` |
| `Job.DebugWidth` | `NeverWrap` (0) | `DefaultDebugWidth` (72) |
| missing `.err` golden | debug stream must be empty | case has no `.err` expectation |
| `-update` | permitted | refused |
| skips when absent | no, it is a failure | yes |

Both skip when `ml1.Run` reports `ml1.ErrNoEngineSource`, which means the LOWL source of ML/I is not
on this machine. It cannot be committed here, so `cmd/fetchtestdata` fetches it along with the
corpora, into `.downloads/lowlml1/`, which is one of the places the engine looks.

The local corpus passes. The upstream corpus does not, and is diagnostic rather than a gate: see
[the version skew](../explanation/running-ml1-on-the-lowl-vm.md).

## The other corpora

Three more golden corpora exist and are not described here, because they are about the L route
rather than about the processor. `pkg/l/testdata` holds the front end's, `pkg/l/lmap/testdata` the
L-map's, and each has its own `README.md`; the harnesses are the same shape as this one down to the
"a missing golden means the stream must be empty" rule and the refusal to create one. `pkg/ml1`
holds one that spans both: `TestLBackendMatchesAIG` runs the local corpus below through an engine
translated out of L and through the published LOWL engine and requires both streams to agree byte
for byte, which is the acceptance test for the whole L route. See
[the L-map](../explanation/the-l-map.md).

## The differential suite

`TestExamplesAgainstOracle` in `pkg/ml1/examples_test.go` is a third suite with **no golden files at
all**. It runs the example programs in `testdata/` through `ml1.Run` and through the oracle and
requires the two to agree, byte for byte, on both streams. The expectation is whatever the oracle
produces, so nobody has to decide in advance what the right answer is — which is what lets these be
real programs rather than cases written to exercise one construct.

| | |
|---|---|
| test | `TestExamplesAgainstOracle` |
| directory | `testdata` (gitignored; Rosetta Code, CC BY-SA) |
| expectation | the oracle, run over the same files |
| comparison | byte for byte, on the output **and** the debugging stream |
| `Job.DebugWidth` | `DefaultDebugWidth` (72), and `Workspace` is `DefaultWorkspace` — the `cmd/ml1` defaults, which is the configuration the oracle is being compared under |
| skips when absent | yes, on the oracle and on the examples alike |

The suite is a table in the test rather than a directory walk, because an example may read a second
input stream and only the table knows which file that is. An example present in the directory but
absent from the table is a failure — a program nothing runs is the state this suite exists to
prevent — so adding one means adding a line there.

Two entries carry a `skew` note and are required to **differ**: `bitwise-operations` and
`csv-to-html` reduce the two non-newline upstream failures to small cases, and each uses a construct
`ml1ajb.lwl` has never heard of. If either starts agreeing, the test fails and says so, because that
means the feature arrived and the note is stale.

`testdata/run-examples.sh` runs the same comparison against the built `cmd/ml1` binary and prints a
table. It covers what the Go test cannot — that the command line wires the streams up correctly —
and stays useful for that reason; the engine itself is now covered by `go test ./...`.

## File layout

A case is `NAME.ml1` with `NAME.out` beside it, and optionally `NAME.err` and `NAME.lst`.

| suffix | holds |
|---|---|
| `.ml1` | the input, read as the last input stream |
| `.out` | expected output stream 1 |
| `.err` | expected debugging stream |
| `.lst` | expected listing, the stream `S20` controls |
| `.2.ml1` … `.5.ml1` | further input streams for that case, not cases themselves |

Cases are discovered, not listed: every `*.ml1` in the directory that is not the prelude, is not
one of a case's extra input streams, and has a `.out` beside it. A corpus directory that exists but
yields no cases is a failure, so a botched extraction cannot look like success.

A case that switches input streams with `MCSET S10=n` needs somewhere to switch to, so it may have
`NAME.2.ml1` through `NAME.5.ml1` beside it — five is all ML/I accepts (AA.2). They are fed in
order after the case's own file and stop at the first gap, so `NAME.ml1` is stream 1 and
`NAME.2.ml1` is stream 2. That numbering holds only for a corpus with no prelude; the upstream
corpus has one, which makes its case file stream 2, so the harness fails on the combination rather
than misnumber the streams quietly. `streams` is the only case using this.

`.out` and `.err` are the upstream names, kept so one runner serves both corpora. `.lst` is ours:
the upstream harness never asks for a listing, so a missing `.lst` there says nothing and the
stream is not compared at all. The root `.gitignore` deliberately **does not** ignore `*.out`; it
used to, which would have silently dropped every golden file. `.gitattributes` pins `*.ml1`,
`*.out`, `*.err`, and `*.lst` to `text eol=lf`, which is what makes byte-exact comparison
portable.

## Where the debugging stream is asserted

No case in the local corpus has an `.err` golden, and none may: the wording of a system message is
upstream's expression, so a golden file would be a machine-readable copy of it. Every local case
therefore asserts only that the stream is **empty**, which is what a clean run produces because
`S18` starts at zero.

The diagnostic path is covered in ordinary Go tests instead:

| test | covers |
|---|---|
| `pkg/ml1/debug_test.go` | `MCNOTE` with and without its context print-out (`S4`), the warning marker and the message `S3` suppresses, an error aborting the construction it was found in, and the `S18` end-of-process report |
| `pkg/ml1/fatal_test.go` | the four fatal conditions of AA.4.1 — the `S12` quota boundary, an illegal `S10`, a rewind that fails, and an output file that cannot be written — and the shape of the print-out each produces |
| `pkg/ml1/storage_test.go` | the diagnostic written when the workspace runs out |

Those assert substrings, not whole streams, for the same licence reason — with one exception.
`MCSET S4=1` makes `MCNOTE` write its argument and nothing else, so that one case is compared byte
for byte against text this repository wrote. It is the only slice of this stream where an exact
comparison is available.

## `corpus` fields

| field | meaning |
|---|---|
| `name` | used in test names and messages |
| `dir` | holds the inputs and the golden files |
| `prelude` | input stream 1 prepended to every run; `""` for none |
| `equal` | `equalExact` or `equalIgnoringSpaceChange` |
| `debugWidth` | passed through as `Job.DebugWidth` |
| `optional` | extensions whose golden may be absent, in which case that stream must be empty |
| `writable` | `-update` may rewrite this corpus |
| `unstable` | case name to reason; its `.err` is not compared |

## `diff -b` semantics

`equalIgnoringSpaceChange` reproduces what `diff -b` does, checked differentially against
`/usr/bin/diff` over 20000 generated pairs:

1. Split on `\n`. A trailing newline terminates the last line rather than starting an empty one.
2. Per line: drop the run of blanks at the end, then collapse every other run of blanks to a single
   space. A run at the *start* collapses to one space rather than disappearing.
3. Compare line by line. A blank line is still a line.

So the amount of whitespace never matters and its presence always does:

| | |
|---|---|
| `a b` vs `a  b` vs `a\tb` | equal |
| `foo` vs `foo   ` | equal |
| `   ` vs `` | equal |
| final newline present vs absent | equal |
| `foo` vs ` foo` | **differ** |
| `ab` vs `a b` | **differ** |
| an inserted blank line | **differ** |

Carriage returns are folded into the trailing trim, which is not part of `-b`; it lets a golden file
with DOS line endings compare against Unix output.

There is one deliberate divergence. When a file's last line has no terminating newline *and* carries
trailing blanks, BSD `diff` stops ignoring those blanks. That looks like an artefact, GNU diff need
not share it, and no golden file can reach it — all 22 upstream golden files end with a newline.
`TestCompare` pins our behaviour so a change to it is deliberate.

## `Job.DebugWidth`

The column at which the debugging stream is hard-wrapped, mid-word.

- `NeverWrap`, which is `0` **and the zero value**, emits every line whole.
- `DefaultDebugWidth` is `72`, matching the reference implementation.

The zero value means "never wrap" rather than "use the default", so a `Job` built by hand does not
transform what the engine writes unless asked. The 72 therefore lives in two places that both set it
explicitly: `cmd/ml1`'s config, and `upstreamCorpus()`.

## `-update`

```sh
go test ./pkg/ml1 -update
```

Rewrites golden files for the `local` corpus only.

- The `upstream` corpus skips under `-update` rather than being rewritten. Those golden files are
  the specification, not a record of what we produce. It skips rather than fails so that this
  command stays usable for the corpus it *is* meant to update.
- It **refuses to create** a golden file that does not exist. Write it by hand first; `-update`
  exists to refresh one after a reviewed behaviour change, not to enshrine current behaviour.
- The flag is registered by this package only, so `go test ./... -update` fails elsewhere with
  "flag provided but not defined". Name the package.

After a run with no behaviour change, `git status` must be clean. If `-update` rewrites files that
already pass, the comparator and the writer disagree about normalization.

## `cmd/fetchtestdata`

| flag | effect |
|---|---|
| `-corpus name` | `required` (default), `all`, or a corpus name |
| `-verify` | check what is on disk, never use the network |
| `-force` | download again even when the corpus verifies |
| `-dest dir` | default `testdata/upstream` under the module root |
| `-cache dir` | default `.downloads/cache` under the module root |
| `-print-manifest file` | print a manifest entry for an archive to stdout |

It refuses to extract anywhere not covered by a `.gitignore` whose first line is `*`, so the
upstream suite cannot land somewhere git would track it. Archives are read entirely into memory and
every member is verified before anything is written, so a bad download cannot leave a partial
corpus. Member names that are absolute, non-canonical, or contain `..` are rejected, as are
non-regular members.

### `manifest.json`

```json
{
  "version": 1,
  "corpora": [
    {
      "name": "tests-ac",
      "url": "https://www.ml1.org.uk/tgz/tests-ac.tar.gz",
      "format": "tar.gz",
      "sha256": "63a3759f...",
      "size": 9849,
      "dest": "tests-ac",
      "optional": false,
      "files": [{ "name": "alter.ml1", "size": 1008, "sha256": "..." }]
    }
  ]
}
```

`format` is `tar.gz` or `zip`. `dest` is relative to the destination directory. `optional` keeps an
archive out of the default fetch. `files` lists every member, so a partial extraction or a later
hand-edit is detectable.

## Exit statuses

`Result.ExitStatus()` implements what the operating instructions specify:

| status | meaning |
|---|---|
| 0 | clean |
| 254 | the process finished but reported errors (`S5` non-zero) |
| 255 | a fatal error ended the process early |
