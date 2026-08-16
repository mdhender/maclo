# ML/I

A Go port of [ML/I](http://www.ml1.org.uk/), P.J. Brown's general purpose macro processor.

It is a port by a route that is worth stating up front, because everything else follows from it.
ML/I is distributed as source written in **LOWL**, a low-level machine-independent language. Rather
than translate that source into Go by hand, this repository implements LOWL — a scanner, parser,
assembler and virtual machine — and *runs the original source on it*. What you get is ML/I as
P.J. Brown wrote it, executing on a machine written in Go.

The practical consequence is that the processor is not in this repository, and cannot be: the LOWL
source is copyright P.J. Brown and R.D. Eager. Its licence permits **building** that source into a
program and forbids redistributing either the source or the program built from it. So the source is
fetched on the machine that builds, and no binary from here can be handed to anyone else.

## Two commands

| | |
|---|---|
| `ml1` | follows [Appendix AA](https://www.ml1.org.uk/htmldoc/ml1appaa.html) of the ML/I user's manual option for option, and finds its engine **on disk** at run time. Use it where being a drop-in for the reference implementation matters. |
| `maclo` | ordinary flags, and the engine is **built into the binary**. Use it otherwise. |

They share the whole processor; they differ only in the command line and in where the engine comes
from. `ml1` is not free to be improved — that is the point of it — and `maclo` is where anything
better goes.

## Build

```sh
git clone https://github.com/mdhender/maclo && cd maclo
go run ./cmd/fetchtestdata     # the engine and the test corpora; both gitignored
go build ./cmd/maclo ./cmd/ml1
```

The fetch writes `pkg/ml1/engines/ml1ajb.lwl`, which is the directory `//go:embed` compiles in, and
`.downloads/lowlml1/`, which is where `ml1` looks at run time.

```sh
cat > hello.ml1 <<'END'
MCSKIP MT,<>
MCDEF GREET AS <Hello, ML/I>
GREET
END

./maclo hello.ml1        # Hello, ML/I
./maclo --engines        # ml1ajb  AJB  57333 bytes  (default)
```

`go install` also works, but note what it gets you: the module on the Go proxy has an empty engines
directory, so the build compiles and carries **no engine**. `maclo` says so and explains how to fix
it; `ml1` falls back to `ml1 --fetch-engine`, which installs a `.lwl` in a per-user directory at run
time. Full instructions are in [Install ML/I](docs/how-to/install-ml1.md).

## Status

Not yet released, and not yet tagged. It works.

| suite | what it is | result |
|---|---|---|
| `TestGoldenLocal` | 25 cases written here from the manual | **25/25**, byte for byte |
| `TestExamplesAgainstOracle` | 17 Rosetta Code programs, against the reference implementation | **17/17** |
| `TestLOWLTEST` | the LOWL kernel conformance program, L4A | passes |
| `TestGoldenUpstream` | the 11-case suite from ml1.org.uk | 1/11 — **see below** |

**`go test ./...` fails on purpose, in exactly one place.** `TestGoldenUpstream` is red and is meant
to be. The engine is the 1986 LOWL source, which is the newest one published; the golden files in
that suite were recorded from a later implementation known as CKQ. Ten of the eleven cases differ on
that version skew and nothing else:

- Seven differ **only by a surplus blank line**, traced to three `MESS` literals in the source we
  run, where CKQ has one `$` fewer.
- One (`errors`) adds a spacing difference in the context display.
- `strings` uses `MCCVAR`, which does not exist in the 1986 source.
- `macrotim` uses bitwise `&` and `|` in a macro expression, which the 1986 `GETEXP` rejects.

None of it is a defect in the port, and none of it is fixable by porting harder — closing it means
implementing CKQ's changes, which is a different project. It is left failing rather than skipped so
that it stays measured. **Anything else red is a regression.** The analysis is in
[BURNDOWN.md](BURNDOWN.md) and
[running ML/I on the LOWL VM](docs/explanation/running-ml1-on-the-lowl-vm.md).

## How it runs

```
ml1ajb.lwl
  → scanner    (pkg/lowl/scanner)     bytes → tokens
  → cst        (pkg/lowl/cst)         tokens → one node per source line
  → ast        (pkg/lowl/ast)         cst nodes → Op + typed parameters
  → assembler  (pkg/lowl/assembler)   ast → a VM with core populated
  → vm.Run     (pkg/lowl/vm)          executes until HALT
```

`pkg/ml1` supplies what LOWL leaves to the host — the input and output streams, the workspace, the
S-variables, the error print-outs — and drives the machine. `ml1.Run` does all of it in memory, from
buffers, writing nothing to disk and printing nothing; two tests hold it to that.

| | |
|---|---|
| `cmd/ml1` | the macro processor, Appendix AA compatible, engine found on disk |
| `cmd/maclo` | the macro processor, modern flags, engine built in |
| `cmd/lasm` | the LOWL assembler and VM on their own, with listings for every stage |
| `cmd/fetchtestdata` | fetches the engine and the test suites into a checkout |
| `pkg/ml1` | the ML/I front end, host boundary, MD-logic, and the embedded engines |
| `pkg/lowl` | the LOWL scanner, parser, assembler and VM |
| `pkg/postfix` | infix → postfix, for the assembler's `OF(...)` expressions |
| `internal/fetch` | download and digest verification, shared by the two commands |

## Documentation

- [Install ML/I](docs/how-to/install-ml1.md) — get a working `maclo` or `ml1`
- [Fetch the upstream sources](docs/how-to/fetch-the-upstream-sources.md) — set up a checkout
- [Golden tests](docs/reference/golden-tests.md) — the corpora and how they are compared
- [Running ML/I on the LOWL VM](docs/explanation/running-ml1-on-the-lowl-vm.md) — the source
  versions, the LOWL extensions, the host boundary, the memory layout
- [The upstream test suite](docs/explanation/upstream-test-suite.md) — why it is not committed
- [`pkg/lowl/README.md`](pkg/lowl/README.md) — the VM and the full opcode table

## Developing

```sh
go run ./cmd/fetchtestdata   # the engine and the test suite; both are gitignored
go test ./...                # green except TestGoldenUpstream, as above
```

Without that fetch there is no processor, so the tests that need one **skip** rather than fail. A
skip and a failure are different things; check which you are looking at. A tree with no engine still
compiles, which is deliberate and is why `pkg/ml1/engines/README.md` is tracked: `//go:embed` needs
one non-hidden file in that directory to exist.

## Licence and copyright

The Go code here is [MIT licensed](LICENSE), copyright Michael D Henderson.

**ML/I is not.** ML/I, its LOWL and L sources, its test suite and its documentation are copyright
P.J. Brown and R.D. Eager, and are not redistributed by this repository. The licence permits
building the source into a program, which is what `//go:embed` does, and forbids redistributing the
source or that program — so nothing upstream is committed here, and **a binary built from this tree
may not be passed on**. There are no release downloads for that reason. What is committed instead is
[`internal/fetch/manifest.json`](internal/fetch/manifest.json), which records the URL, size and
SHA-256 of each upstream file: facts about them rather than copies of them. Everything ML/I is
fetched from <http://www.ml1.org.uk/>, which is the place to get it and where the manual lives.

## References

- [The ML/I home page](http://www.ml1.org.uk/) — sources, manual, and every port
- [ML/I user's manual](https://www.ml1.org.uk/htmldoc/ml1man.html)
- [Computer Conservation Society, Resurrection 84](https://computerconservationsociety.org/resurrection/res84.htm#d)
  — Bob Eager on ML/I's history
