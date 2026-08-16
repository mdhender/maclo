# Fetch the upstream sources

Two things the tests need cannot live in this repository: the ML/I test suite, and the LOWL source
of ML/I itself, which is the processor the tests run. Both come from <http://www.ml1.org.uk/> and
both are fetched by one command. Until you run it, the tests that need them skip.

For *why* they cannot be committed, see [the upstream test suite](../explanation/upstream-test-suite.md).

## Fetch them

```sh
go run ./cmd/fetchtestdata
```

That downloads each archive, checks it against the digests in `internal/fetch/manifest.json`,
verifies every file it contains, and only then writes them out:

| what | goes to | why there |
|---|---|---|
| `lowlml1` — the LOWL source of ML/I | `.downloads/lowlml1/` | it is the engine, and that is where `ml1.Run` looks for it |
| `tests-ac` — the test suite | `testdata/upstream/tests-ac/` | it is test data |

The archives are cached in `.downloads/cache/`, and every one of those directories is ignored by
git. The command refuses to write anywhere else: a destination that is not inside a directory whose
`.gitignore` starts with `*` is an error, not a warning.

Confirm it worked:

```sh
go test ./pkg/ml1 -v
```

Run it again whenever you like. A second run finds everything already in place and does nothing:

```
lowlml1: up to date (2 files)
tests-ac: up to date (41 files)
```

## Read what the tests tell you

`--- SKIP ... is not present; run: go run ./cmd/fetchtestdata`
: You have not fetched the suite yet. Do that.

`--- SKIP ... cannot read the LOWL source of ML/I`
: The engine is missing. ML/I is run from its own LOWL source rather than from a translation of it,
  so without that source there is no processor to test. Run the same command. This skip clears
  itself once the source is there; nothing needs editing.

`--- FAIL ... golden mismatch at line N`
: A real difference. The message quotes both sides with `%q`, so whitespace you cannot see on
  screen is still visible in the output.

## Work offline

To check what is already on disk without touching the network:

```sh
go run ./cmd/fetchtestdata -verify
```

This also succeeds when the archive is only in `.downloads/cache/`, since a cached archive whose
hash matches is used in preference to downloading again.

## Fetch the optional archives

Two more are known but not fetched by default, because nothing is gated on either:

- `lowltest` — LOWLTEST, the LOWL kernel conformance program. It checks the VM against the kernel
  manual rather than against ML/I. `TestLOWLTEST` in `pkg/lowl/vm` assembles and runs it once it is
  here and skips when it is not, so fetching it turns that test on and nothing else changes.
  `lasm` will also run it by hand, leaving the report in `vm_stdmsg.txt`.
- `tests-r.zip` — an **older generation of the same tests**, not a re-encoding of the tar one: some
  of its golden files differ in ways `diff -b` will not forgive.

```sh
go run ./cmd/fetchtestdata -corpus all      # both
go run ./cmd/fetchtestdata -corpus lowltest # just one
```

## When the archive hash does not match

You will see something like:

```
tests-ac: the archive is not what the manifest describes
  url      https://www.ml1.org.uk/tgz/tests-ac.tar.gz
  expected 63a3759f...
  actual   1f4b90c2...
```

Nothing has been written. **Do not edit `manifest.json` to make this pass.** The digests are the
only thing standing between a change upstream and a silent change to what we consider correct.
Instead:

1. Download the archive by hand and look at what actually changed.
2. If the change is legitimate, regenerate the entry and review the diff:

   ```sh
   go run ./cmd/fetchtestdata -print-manifest /path/to/tests-ac.tar.gz
   ```

   This prints JSON to standard output for you to paste in. It never edits `manifest.json` itself,
   because accepting a change upstream should be a deliberate act by a person.
3. Commit the manifest change on its own, so the diff is reviewable.

## Get the reference implementation

You only need this to **add a case to the local corpus**, not to run the tests. It is the oracle
that produces golden files, so it has to be the right build:

**ML/I on Apple (Intel) under macOS — implementation version 4.13, ML/I version CKQ**, by Bob
Eager, from <https://www.ml1.org.uk/impl-ac.html> ("ML/I executable file").

Save it as `.downloads/ml1`, which git ignores — it is no more redistributable than the test suite.
Then confirm you have that exact build:

```sh
chmod +x .downloads/ml1
.downloads/ml1 -v </dev/null                       # ml1: macOS version 4.13 (CKQ)
shasum -a 256 .downloads/ml1
# 4ab419fafe8ecdcfd26c9701f7d15f74bb0a00deca4579ee8009c95601843fae
```

It is an x86-64 binary; on Apple Silicon it runs under Rosetta 2. ML/I is a family of ports and they
do not all agree byte for byte, so a different platform build is **not** a drop-in substitute. If
you have to use one, check it against the upstream suite first:

```sh
cd testdata/upstream/tests-ac
for n in alter errors escalate macrotim names overflow override s skipins strings structur; do
  ../../../.downloads/ml1 -v sets18.ml1 $n.ml1 -o /tmp/$n.out -d /tmp/$n.err 2>/dev/null
  if cmp -s $n.out /tmp/$n.out && cmp -s $n.err /tmp/$n.err
    then echo "$n ok"
    else echo "$n DIFFERS"
  fi
done
```

The 4.13 (CKQ) build reproduces all 22 files byte for byte. Anything less and its output is not
trustworthy as a golden file.

## Remove what was fetched

```sh
rm -rf testdata/upstream          # the corpora
rm -rf .downloads/lowlml1         # the engine
```

The tests go back to skipping. Nothing else depends on either, and `go run ./cmd/fetchtestdata`
puts them back.
