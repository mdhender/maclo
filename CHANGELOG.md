# Changelog

Notable changes to this port. The format follows [Keep a Changelog](https://keepachangelog.com/),
and versions follow [semantic versioning](https://semver.org/) once there is one — nothing is
tagged yet.

Versions here are versions of **the port**. ML/I's own version is a property of the LOWL source the
engine runs, which is AJB; see [the README](README.md#status) for what that implies.

## Unreleased

Everything so far. This will become 0.1.0.

### The processor

- ML/I runs, by implementing LOWL and executing the distributed source on it rather than by
  translating that source into Go. `cmd/ml1` takes the options of
  [Appendix AA](https://www.ml1.org.uk/htmldoc/ml1appaa.html): up to 5 input files and 4 output
  files, `-w` workspace, `-l` listing, `-d` debugging stream, and the documented exit statuses.
- `pkg/ml1` is the library the command is a wrapper over. `ml1.Run` works entirely from buffers and
  `io.Writer`s: it writes no files, prints to no stream it was not given, and is held to that by
  both a behavioural and a static test.
- Fatal conditions produce ML/I's own four-part error print-out — prologue, message, context walk,
  and the line that says the process stopped — rather than a Go error string.
- The `S18` end-of-process report, the `S12` debugging quota, `S19`/`S20` listings, `S6`, permanent
  and temporary variables, multiple input streams, stream switching and rewinding all work.

### Getting the engine

- `ml1 --fetch-engine` downloads the LOWL source of ML/I from ml1.org.uk into a per-user directory,
  verifying every file against a SHA-256 recorded in `internal/fetch/manifest.json` before writing
  anything. This is what makes an installed binary usable; before it, the engine was only ever found
  in a developer's checkout.
- `ml1 --engine` reports every path that is searched and which one answers.
- The search order is `-s`, then `$ML1_LOWL_SOURCE`, then `$ML1_HOME` or the per-user directory,
  then `.downloads/` in a checkout.
- `go run ./cmd/fetchtestdata` still populates a checkout with the engine and the test corpora
  together.

### Tests

- `TestGoldenLocal` — 25 cases written here from the manual, compared byte for byte against output
  from the reference implementation. All pass.
- `TestExamplesAgainstOracle` — 17 Rosetta Code programs run through this engine and the reference
  implementation and required to agree on both streams. All pass; two are required to *differ*, and
  are the minimal reproductions of the version skew below.
- `TestLOWLTEST` — the LOWL kernel conformance program, L4A.
- `TestGoldenUpstream` — the suite from ml1.org.uk. **Fails on purpose**: its golden files were
  recorded from a later implementation (CKQ) than the newest published source (AJB). Ten of eleven
  cases differ on that skew alone, seven of them by a single blank line. It is left failing rather
  than skipped so that it stays measured.

### Known limitations

- `MCCVAR` and the bitwise `&` and `|` of a macro expression are absent from the 1986 source and so
  are absent here.
- The engine is a runtime dependency and cannot be embedded, shipped, or vendored.
