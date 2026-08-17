# Changelog

Notable changes to this port. The format follows [Keep a Changelog](https://keepachangelog.com/),
and versions follow [semantic versioning](https://semver.org/) once there is one — nothing is
tagged yet.

Versions here are versions of **the port**. ML/I's own version is a property of the LOWL source the
engine runs — AJB, AIH or AIG, whichever a build was given; see [the README](README.md#status) for
what that implies.

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

### The L front end

- `pkg/l` scans, parses and name-checks **L**, the machine-independent language ML/I's logic is
  written in, as opposed to LOWL, the low level language it is distributed as. There is no back
  end: this generates no code and runs nothing.
- Two commands drive it, split the way `cmd/lasm` and `cmd/maclo` are. `cmd/macl` reports on a
  program — `check summary list symbols source run` — and is where a back end will arrive;
  `macl run` reserves the word and, for now, runs the front end and then says what is missing and
  what to use instead. `cmd/lcheck` reports on the parse and dumps the stages.
- `l.Summary` counts a program: statements by kind, the SECTIONs in order, how deep the two
  restricted nestings go, and the names by kind. It lives in `pkg/l` and not in a command, so
  `TestML1AIE` can assert every field of it against the real 2,510 lines rather than against a
  walk that only a command runs.
- The whole 2,510-line L source of ML/I (version AIE) parses with no lexical, syntactic or
  structural diagnostic, and resolves with exactly one — a label in the file spelt with a letter
  too many, which the LOWL distribution confirms is a typo in AIE rather than a gap here.
- The listing is a canonical re-render, indented to show the nesting, and it round-trips that whole
  file byte for byte.
- Unlike the LOWL front end the ast is a tree: one Go type per statement, compound statements
  holding their bodies, and the five closing statements folded onto what they close. `pkg/l/stmt`
  is a single table rather than the enum, stringer and lookup map the LOWL side keeps in step by
  hand, and every stage accumulates diagnostics instead of stopping at the first.
- Nothing under `pkg/l` writes a file or touches a process stream; every listing takes an
  `io.Writer`, and the only callers of `os.Create` are `cmd/macl` and `cmd/lcheck`, each on a path
  the user named. Both have a test that runs every subcommand from a temporary directory and
  requires it to be empty afterwards.

### Getting the engine

- **The engine can be built in.** The licence on the LOWL source permits compiling it into a
  program; it forbids redistributing the source or the program. So `pkg/ml1/engines/` is embedded
  with `//go:embed` and holds nothing tracked but a `.gitignore` that denies everything and a
  `README.md` that exists so the build works when the directory is otherwise empty. A tree with zero
  engines compiles.
- `cmd/maclo` is a second front end: ordinary Go flags, and the engine comes from inside the binary.
  `--engines` lists what a build carries, `--engine` selects one by name or by path, and the newest
  by file name is the default. A build with none says so, at length, rather than failing obscurely.
- `cmd/ml1` is unchanged and stays that way. It follows Appendix AA and finds its engine on disk,
  and `Run` does not consult the embedded engines unless a caller asks for one by name — so what
  `ml1` does is independent of what the binary was built with.
- **All three published LOWL sources are fetched and embedded** — AJB, AIH and AIG. Which archives
  carry an engine is a `"engine": true` fact in `internal/fetch/manifest.json` rather than a name
  matched in code, so adding a version of ML/I is a manifest entry and no code change.
- `go run ./cmd/fetchtestdata` copies each engine's `.lwl` into the embed directory, so one command
  still prepares a checkout completely.
- The three agree on all 25 local corpus cases, byte for byte on both streams. They differ in the
  wording of diagnostics: AIH and AIG write the context print-out in capitals where AJB does not.
- A binary built from this tree **may not be distributed**. There are no release downloads.
- `ml1 --fetch-engine` downloads the LOWL source of ML/I from ml1.org.uk into a per-user directory,
  verifying every file against a SHA-256 recorded in `internal/fetch/manifest.json` before writing
  anything. This is what makes an installed binary usable; before it, the engine was only ever found
  in a developer's checkout.
- `ml1 --engine` reports every path that is searched and which one answers.
- The search order is `-s`, then `$ML1_LOWL_SOURCE`, then `$ML1_HOME` or the per-user directory,
  then `.downloads/` in a checkout.

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

- `pkg/l/testdata` is an original L corpus, written from the L manual, compared byte for byte, with
  one case per lexical trap and one per diagnostic. `TestML1AIE` runs the front end over the real L
  source and skips when it has not been fetched; `TestRoundTrip` requires every listing to read back
  as itself. The `lml1` manifest entry is what fetches the source.

### Known limitations

- `MCCVAR` and the bitwise `&` and `|` of a macro expression are absent from every published LOWL
  source and so are absent here.
- The engine can be built into a binary but not shipped in one, and not vendored. Every machine that
  wants a working processor has to fetch it.
