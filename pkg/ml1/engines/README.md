# Engines

The LOWL sources of ML/I that get built into the binary. Drop `*.lwl` files here and they are
embedded by `//go:embed` in `pkg/ml1/embed.go`; `go run ./cmd/fetchtestdata` puts all three of the
published versions here for you — `ml1ajb.lwl`, `ml1aih.lwl` and `ml1aig.lwl`.

**Nothing in this directory may be committed.** The sources are copyright P.J. Brown and
R.D. Eager. Their licence permits building them into a program — which is what the embed does — but
not redistributing the source or a program compiled from it. So the `.gitignore` beside this file
denies everything and then allows back exactly two names, itself and this README, rather than
listing what to exclude: a pattern that has to be *added to* for each new file is a pattern that
will one day be forgotten.

The practical consequences:

- A clone of this repository has an **empty** directory here and compiles anyway. That is the
  point: `//go:embed engines` needs one non-hidden file to exist, which is why this README is
  tracked and why deleting it breaks the build.
- The module on the Go proxy also has an empty directory here, so `go install` produces a binary
  with **no engine in it**. `maclo` says so loudly; `ml1` falls back to looking for a `.lwl` on
  disk, which is what it has always done.
- A binary built with sources here cannot be handed to anyone else. Building is fine. Distributing
  the result is not.

## Naming

The file name carries the version, and that is what orders them: `ml1aig` is older than `ml1aih` is
older than `ml1ajb`, and the newest is the default. `maclo --engines` lists what a given binary
actually has, which is the only reliable way to tell.

The three agree on everything the local corpus covers — all 25 cases produce identical output and
identical debugging streams. Where they differ is the wording of diagnostics: AIH and AIG write the
context print-out in capitals (`DETECTED IN`, `LINE 3 OF SOURCE TEXT`) where AJB writes it in lower
case. Anything golden is therefore recorded against AJB.
