# Install ML/I

This gets a working `ml1` on your machine. It takes two commands, because the program and the
processor are separate things: ML/I is distributed as source written in LOWL, this port runs that
source rather than a translation of it, and the licence on that source forbids shipping a copy of
it. So the binary is installed from here and the engine is fetched from
<http://www.ml1.org.uk/>.

For *why* it works that way, see
[running ML/I on the LOWL VM](../explanation/running-ml1-on-the-lowl-vm.md).

## Install the command

```sh
go install github.com/maloquacious/ml_i/cmd/ml1@latest
```

Go 1.23 or later. The binary lands in `$(go env GOPATH)/bin`; put that on your `PATH` if it is not
there already.

## Fetch the engine

```sh
ml1 --fetch-engine
```

```
fetching https://www.ml1.org.uk/tgz/lowlml1.tar.gz
lowlml1: 2 files in /Users/you/Library/Application Support/ml1
engine ready: /Users/you/Library/Application Support/ml1/ml1ajb.lwl
```

Every file is checked against a SHA-256 recorded in this repository before anything is written, so a
damaged download or a change upstream stops the command rather than installing something unexpected.
Run it again whenever you like; a second run finds the engine already in place and does nothing.

Confirm what will be used:

```sh
ml1 --engine
```

The `->` marks the file that answers.

## Run a program

```sh
cat > hello.ml1 <<'END'
MCSKIP MT,<>
MCDEF GREET AS <Hello, ML/I>
GREET
END

ml1 hello.ml1
```

By default the output goes to the standard output and anything ML/I has to say about the run goes to
the standard error. `ml1 --help` lists the rest; the options come from
[Appendix AA](https://www.ml1.org.uk/htmldoc/ml1appaa.html) of the ML/I user's manual, which is also
the reference for the language itself.

## Put the engine somewhere else

The search order is:

1. `-s <file>` on the command line
2. `$ML1_LOWL_SOURCE`, naming the file
3. `$ML1_HOME`, or the per-user directory below, holding `ml1ajb.lwl`
4. `.downloads/lowlml1/ml1ajb.lwl`, relative to the working directory — a checkout of this
   repository

The per-user directory is `~/Library/Application Support/ml1` on macOS, `%AppData%\ml1` on Windows,
and `$XDG_DATA_HOME/ml1` — `~/.local/share/ml1` by default — elsewhere.

To keep several versions of ML/I side by side, fetch once and select with `-s`:

```sh
ml1 -s /opt/ml1/ml1ajb.lwl hello.ml1
```

`--fetch-engine` writes to `$ML1_HOME` when that is set, so pointing it at a directory of your own
installs there instead:

```sh
ML1_HOME=/opt/ml1 ml1 --fetch-engine
```

It will not write into a git working tree unless a `.gitignore` there ignores everything, which is
what keeps the upstream source from being committed by accident.

## When it will not run

`ml1: cannot read the LOWL source of ML/I`
: No engine was found. The message lists every path that was tried. Run `ml1 --fetch-engine`, or
  name a source with `-s`.

`ml1: no home directory to install into`
: `--fetch-engine` could not work out where to put the file. Set `$ML1_HOME` to a directory you
  want it in and run it again.

`the archive is not what the manifest describes`
: The download did not match the recorded digest, and nothing was written. Either the download is
  damaged — try again — or upstream has changed, which needs a person to look at it. See
  [fetch the upstream sources](fetch-the-upstream-sources.md).

## Build from a clone instead

If you are working on the port rather than using it, fetch the engine and the test suite together
and leave both in the checkout:

```sh
git clone https://github.com/maloquacious/ml_i
cd ml_i
go run ./cmd/fetchtestdata
go run ./cmd/ml1 hello.ml1
```

That puts the engine in `.downloads/lowlml1/`, which is the fourth entry in the search order, so no
environment variable is needed. See [fetch the upstream sources](fetch-the-upstream-sources.md) for
the rest of what a checkout needs.
