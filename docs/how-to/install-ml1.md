# Install ML/I

This gets a working macro processor on your machine. There are two commands and two ways to give
them an engine, and which you want depends on what you are doing:

| you want | use | the engine comes from |
|---|---|---|
| a drop-in for the reference implementation | `ml1` | a `.lwl` file on disk, at run time |
| anything else | `maclo` | inside the binary, put there at build time |

Either way the engine is fetched from <http://www.ml1.org.uk/> rather than shipped. ML/I is
distributed as LOWL source, this port runs that source rather than a translation of it, and the
licence permits building the source into a program but not redistributing the source or the program.
So nothing here carries it, and **nothing you build here may be passed on**.

For *why* it works that way, see
[running ML/I on the LOWL VM](../explanation/running-ml1-on-the-lowl-vm.md).

## Build with the engine inside (`maclo`)

```sh
git clone https://github.com/maloquacious/ml_i && cd ml_i
go run ./cmd/fetchtestdata
go build ./cmd/maclo
```

Go 1.23 or later. The fetch checks every file against a SHA-256 recorded in this repository before
writing anything, and puts `ml1ajb.lwl` in `pkg/ml1/engines/`, which is the directory `//go:embed`
compiles into the binary.

Confirm what your binary carries:

```sh
./maclo --engines
```

```
ml1ajb       AJB    57333 bytes  (default)
```

Then run a program:

```sh
cat > hello.ml1 <<'END'
MCSKIP MT,<>
MCDEF GREET AS <Hello, ML/I>
GREET
END

./maclo hello.ml1
```

`maclo -h` lists the options. By default the output goes to the standard output and anything ML/I
has to say about the run goes to the standard error.

## Install the AA-compatible command (`ml1`)

```sh
go install github.com/maloquacious/ml_i/cmd/ml1@latest
ml1 --fetch-engine
```

`--fetch-engine` downloads the LOWL source into a per-user directory —
`~/Library/Application Support/ml1` on macOS, `%AppData%\ml1` on Windows, and `$XDG_DATA_HOME/ml1`,
by default `~/.local/share/ml1`, elsewhere. `ml1 --engine` reports every path it searches and marks
the one that answers.

`ml1 --help` lists the options, which follow
[Appendix AA](https://www.ml1.org.uk/htmldoc/ml1appaa.html) of the ML/I user's manual — also the
reference for the language itself.

## Choose a different engine

`maclo` runs the newest engine it was built with. To use another:

```sh
./maclo --engine ml1aih hello.ml1          # another one built in, by name
./maclo --engine /opt/ml1/ml1ajb.lwl hello.ml1   # a file this binary does not carry
```

A `--engine` argument that is neither a built-in name nor an existing file is refused, and the
message lists what the binary does have. To build with several, put more than one `.lwl` in
`pkg/ml1/engines/` and rebuild; the newest by file name becomes the default, so `ml1ajb` wins over
`ml1aih`.

`ml1` selects with `-s <file>`, `$ML1_LOWL_SOURCE`, or `$ML1_HOME`, in that order, and never uses an
embedded engine — it behaves the way its operating instructions say it does whatever the binary was
built with.

## When it will not run

`no ML/I engine is built into this binary`
: `maclo` was built with an empty `pkg/ml1/engines/`. That is what `go install` produces, because
  the module on the Go proxy has no `.lwl` in it — the licence keeps it out. Clone, run
  `go run ./cmd/fetchtestdata`, and `go build ./cmd/maclo`. Or name a source on disk with
  `--engine /path/to/ml1ajb.lwl`.

`ml1: cannot read the LOWL source of ML/I`
: `ml1` found no engine. The message lists every path it tried. Run `ml1 --fetch-engine`, or name
  one with `-s`.

`ml1: no home directory to install into`
: `--fetch-engine` could not work out where to put the file. Set `$ML1_HOME` and run it again.

`the archive is not what the manifest describes`
: The download did not match the recorded digest, and nothing was written. Either it is damaged —
  try again — or upstream has changed, which needs a person to look at it. See
  [fetch the upstream sources](fetch-the-upstream-sources.md).

`pattern engines: cannot embed directory engines: contains no embeddable files`
: `pkg/ml1/engines/README.md` was deleted. `//go:embed` needs one non-hidden file in that directory
  for the build to work with no engines present; restore it from git.
