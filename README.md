# ML/1
The official ML/1 site is [here](http://www.ml1.org.uk/).

# So What Is This?
Just another vanity project.

I ported Strachey's GPM to Go.
That lead to this.

# Tests

```sh
go test ./...                    # note: this fails on purpose, see CLAUDE.md
go run ./cmd/fetchtestdata       # fetch the upstream test suite, once
go test ./pkg/ml1                # the golden tests
```

The ML/I test suite from ml1.org.uk cannot be committed here, so it is fetched on demand and the
tests that need it skip until you do.

* [Fetch the upstream test suite](docs/how-to/fetch-the-upstream-test-suite.md) — how
* [The upstream test suite](docs/explanation/upstream-test-suite.md) — why it is not committed
* [Golden tests](docs/reference/golden-tests.md) — reference

## References

* https://computerconservationsociety.org/resurrection/res84.htm#d

