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
looks like in one real suite: plain
[`sclevine/spec`](https://github.com/sclevine/spec) for structure (no
fork -- `before`/`after`/`context` are one local line, see `spec`'s own
README), [`github.com/woodie/expect`](https://github.com/woodie/expect)
for Gomega-style matchers against plain `*testing.T`, and `gorderly` to
render the result.

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
	spec.Run(t, "Object", func(t *testing.T, describe spec.G, it spec.S) {
		context, before, after := describe, it.Before, it.After

		var obj *myapp.Object

		before(func() { obj = myapp.NewObject(t.Context()) })
		after(func() { obj.Close() })

		describe("DoThing", func() {
			context("with a temp dir", func() {
				before(func() { obj.Dir = t.TempDir() })

				it("succeeds", func() {
					expect(obj.DoThing(), t).To(Succeed())
				})
			})
		})
	})
}
```

`TestObject` passes its closure straight to `spec.Run` -- no separate
named suite function, so there's only one place to look, not two. See
`gorderly`'s own `parse_test.go`/`render_test.go` for this pattern used
against a real parser, not a sketch. Every identifier that reads
lowercase here does so for its own reason: `describe`/`it` are just this
closure's own parameter names (`spec.Run` hands them in positionally, so
nothing stops you naming them however you like); `context`/`before`/
`after` are the same two values under three names, assigned once at the
top of the closure (`context, before, after := describe, it.Before,
it.After`) instead of called as `describe.AsContext()`/`it.Before(...)`/
`it.After(...)` at every site; `t.Context()`/`t.TempDir()` are the
closure's own `t` parameter, reachable from inside `before`/`after` by
ordinary closure capture, no special method needed; and `expect` is a
one-line local alias standing in for `expect`'s own capitalized `Expect`,
since Go requires a dot-imported name to stay capitalized but never
requires that of a parameter or local variable. Pipe the whole thing
through `go test -v ./... | gorderly -fd` and it renders exactly like the
`spec`-only example above -- `gorderly` never knows or cares that
`expect` was involved, it only ever sees `go test -v`'s own output.

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
