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

`gorderly --version`/`gorderly -v` prints the installed version and exits
immediately, without waiting on stdin -- matching `xctidy`'s own
`--version`/`-v`.

## Output styles

Four named styles, each matching a convention from a familiar test runner.
The first three end with the same xcbeautify-style footer; `-fv` ends with
Vitest's own footer shape instead.

| Flag | Convention | Look |
|---|---|---|
|   | Our base formatter | Glyph + `name (N seconds)`, failures add `(FAILED - N)` |
| -fd | RSpec's doc format | Plain colored name, yellow `(PENDING)` for skips |
| -fs | Mocha's spec format | Green `✔` + gray name, red `✗ name (FAILED - N)` |
| -fv | Vitest's own tree | Green `✓ name`, two-toned green `2ms`, red `× name`, dim gray `↓ name` |

`-fv` is [`xctidy`](https://github.com/woodie/xctidy)'s `-fv` counterpart
for the Go Test side -- same glyphs, same millisecond conversion, same
`Tests`/`Duration` footer shape. It currently omits Vitest's `Test Files`
line: XCTest's own Test Suite nesting (a per-class suite wrapped in an "All
tests"/"Selected tests" aggregate suite) hasn't been verified against real
`xcodebuild` output, so a suite-level pass/fail count risks over-counting
the wrapper suites as if they were their own files. `gorderly`'s equivalent
(one line per Go package) had no such ambiguity.

## Writing tests

`gorderly` only ever needs `go test -v`'s raw output -- it renders
whatever nesting your tests already produce, `spec`-based or not. For how
we actually write those tests -- `spec` for structure and lifecycle
hooks, [`expect`](https://github.com/woodie/expect) for matchers, context
nesting, the `subject` pattern, mocking and stubbing -- see
[docs/FRAMEWORK.md](docs/FRAMEWORK.md). `gorderly`'s own tests
(`parse_test.go`, `render_test.go`, `main_test.go`, `version_test.go`,
`config_test.go`) are real examples of that shape, not just a sketch.

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
- A literal `/` in a `describe`/`context`/`it` string is indistinguishable
  from a nesting boundary once it reaches `go test -v`'s output -- `t.Run`
  uses an unescaped `/` as both the subtest hierarchy separator and the
  `-run`/`-bench` regex-splitting delimiter, and [Go's own docs are
  explicit](https://go.dev/blog/subtests) that a literal `/` in a subtest
  name isn't safe. Not something `gorderly` (or any tool parsing `go test`'s
  output) can recover after the fact -- see
  [docs/FRAMEWORK.md](docs/FRAMEWORK.md#a-literal--in-a-description-string)
  for the fix on the spec-author side.

## Development

```
make build    # go build -o gorderly
make install  # builds, then moves the binary to ~/go/bin/
make test     # verbose, dogfoods gorderly on its own suite
make lint     # golangci-lint
make check    # terse: silent on success, full log on failure
```

Cutting a release: bump `gorderlyVersion` in `version.go` by hand before
tagging. Unlike `xctidy`'s `Version.swift` (regenerated from `git describe`
at build time), `gorderly`'s primary install path is `go install
github.com/woodie/gorderly@latest` -- a module-proxy fetch with no `.git`
metadata to describe -- so the version string has to already be correct in
the committed source at the tagged commit.
