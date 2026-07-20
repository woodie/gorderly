# gorderly

[![go.mod version](https://img.shields.io/github/go-mod/go-version/woodie/gorderly)](https://github.com/woodie/gorderly)
[![CI](https://github.com/woodie/gorderly/actions/workflows/ci.yml/badge.svg)](https://github.com/woodie/gorderly/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/woodie/gorderly.svg)](https://github.com/woodie/gorderly/releases/latest)
[![License](https://img.shields.io/github/license/woodie/gorderly.svg)](LICENSE)

![Example Screenshot](docs/example.png)

RSpec style output for plain `go test` -- no BDD framework required.

`gorderly` reads `go test -v`'s raw output directly (the same textual
protocol every other Go tool already parses) and re-renders it as a nested
tree, using the hierarchy `t.Run` subtests already carry in their
`/`-joined names. No `--json-report` round trip, no test-runner dependency,
no third-party BDD DSL to adopt -- if your tests use stdlib `testing` with
nested `t.Run`, `gorderly` already understands them.
[`gotestsum`](https://github.com/gotestyourself/gotestsum) already covers
`dots`/`testname`/`pkgname`/`testdox` well -- `gorderly` exists
for the one format it doesn't: a real deduped, nested tree.

## Installation

```
go install github.com/woodie/gorderly@latest
```

Or build locally:

```
go build -o gorderly
mv gorderly ~/go/bin/
```

## Usage

Pipe `go test -v`'s output straight in:

```
go test -v ./... | gorderly -fd
```

Or let `gorderly` run `go test` for you:

```
gorderly -fd .
gorderly -fd ./...
```

Flags: default (no flag) renders the classic style (glyph + per-test
elapsed time); `-fd` renders RSpec's `-fd` documentation format; `-fs`
renders Mocha/Jest's spec format; `--format documentation` and `--format
spec` are the long forms of `-fd`/`-fs`, matching
[`xctidy`](https://github.com/woodie/xctidy)'s exact flag surface.

`-fv` (long form `--format vitest`) renders
[Vitest](https://vitest.dev)'s own tree-reporter conventions: `✓`/`×`/`↓`
glyphs and a right-justified `Test Files`/`Tests`/`Duration` footer, so
`go test`'s output reads as a continuation of the same terminal a
`vitest run` right above it already printed -- gorderly never touches or
wraps Vitest's own output to get there, it just matches it. This flag is
gorderly-specific, not part of the shared flag surface with `xctidy`
(there's no Vitest equivalent on the XCTest side).

## Writing tests with `spec`

[`sclevine/spec`](https://github.com/sclevine/spec) gives you `when`/`it`
structure with real `Before`/`After` hooks -- it's built entirely on `t.Run`,
so its raw `go test -v` output is exactly what `gorderly` already renders.
`spec` got test organization and per-`it` freshness right years ago; it just
never had a reporter to match. `gorderly` gives it one for free, since both
tools only ever speak plain `go test -v`. This is the minimal shape -- plain
upstream `spec`, no `expect`, no fork -- for a project that just wants
`gorderly`'s renderer with no other opinions. `gorderly`'s own tests
(`parse_test.go`, `render_test.go`) have since moved to the fuller shape
below ("The full toolchain") -- see that section for what they actually
look like today.

```go
spec.Run(t, "GoodTimes", func(t *testing.T, when spec.G, it spec.S) {
	when("today is Friday (5)", func() {
		it.Before(func() { gt = newGoodTimes(5) })

		it("computes tomorrow as Saturday (6)", func() {
			if gt.tomorrowDotw() != 6 {
				t.Errorf("tomorrowDotw = %d, want 6", gt.tomorrowDotw())
			}
		})
	})
})
```
Pipe that through `gorderly -fd` and it renders as a real, deduped, nested tree.

## The full toolchain: `spec` + `expect` + `gorderly`

Each piece stays independent -- `gorderly` only ever needs `go test -v`'s
raw output, `spec` needs no assertion library, `expect` needs no BDD
framework -- but they're built to be used together, and here's what that
looks like in one real suite: [`github.com/woodie/spec`](https://github.com/woodie/spec)
(this account's fork, adding `RunAliased`/`Describe`/`Var[T]`/`it.Context()`/`it.T()`
on top of upstream) for structure, [`github.com/woodie/expect`](https://github.com/woodie/expect)
for Gomega-style matchers against plain `*testing.T`, and `gorderly` to render the result.

```go
package myapp_test

import (
	"testing"

	"github.com/sclevine/spec"
	. "github.com/woodie/expect"
)

// expect is the recommended lowercase alias -- one line, declared once per
// test package (see expect's own README, "Lowercase call sites").
func expect[T any](got T, t testing.TB) Expectation[T] { return Expect(got, t) }

func TestObject(t *testing.T) {
	spec.RunAliased(t, "Object", objectSuite)
}

func objectSuite(t *testing.T, describe, context spec.Describe, it spec.S, before, after func(func())) {
	var obj *myapp.Object

	before(func() { obj = myapp.NewObject(it.Context()) })
	after(func() { obj.Close() })

	describe("DoThing", func() {
		context("with a temp dir", func() {
			before(func() { obj.Dir = it.T().TempDir() })

			it("succeeds", func() {
				expect(obj.DoThing(), t).To(Succeed())
			})
		})
	})
}
```

`TestObject` is a one-liner into a named `objectSuite` function -- see
`lambada`'s own test files for this pattern used against a real HTTP
app, not a sketch. Every identifier that reads lowercase here does so
for its own reason: `describe`/`context`/`it`/`before`/`after` are just
this function's own parameter names (`spec.RunAliased` hands them in
positionally, so nothing stops you naming them however you like), while
`expect` is a one-line local alias standing in for `expect`'s own
capitalized `Expect`, since Go requires a dot-imported name to stay
capitalized but never requires that of a parameter name. Pipe the whole
thing through `go test -v ./... | gorderly -fd` and it renders exactly
like the `spec`-only example above -- `gorderly` never knows or cares
that `expect` was involved, it only ever sees `go test -v`'s own output.

## Limitations

- Tree order follows completion order, which matches declaration order for
  serial tests but can reorder under `t.Parallel()`
- Subtest names have spaces substituted with underscores by `go test`
  itself; `gorderly` reverses that for display, which is imprecise for
  subtest names with genuine underscores in them.
- Package build failures are reported by outcome only, not with the
  underlying compiler error -- run `go vet`/`go build` separately to see why
  a package failed to build.
- `-fv`'s per-leaf millisecond timing is only as precise as `go test -v`'s
  own output, which reports elapsed time to two decimal places of a second
  (e.g. `0.00s`) -- fast subtests will often show a flat `0ms` where Vitest's
  own JS timers would show `2ms`/`4ms`. This is a ceiling in `go test`
  itself, not something `-fv` can recover.

## Development

```
make test    # verbose, dogfoods gorderly on its own suite
make lint    # golangci-lint
make check   # terse: silent on success, full log on failure
```
