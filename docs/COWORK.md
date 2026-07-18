# Working with gorderly

Cross-project conventions (git locks, sandbox toolchain, pushing, comments, code style)
are in `~/workspace/woodie/docs/COWORK.md`.

## What this is

A Go equivalent of `xctidy`, for plain `go test`. Reads `go test -v`'s raw
stdout directly (piped in, or by shelling out to `go test -v` itself when
given a package path) and re-renders it as a nested RSpec `-fd`-style tree,
using the `/`-joined hierarchy `t.Run` subtests already carry in their
names. No BDD framework, no `--json-report` file, no wrapped test-runner
binary -- the whole design is "what `xctidy` does for `xcodebuild`, applied
to `go test`."

## How this came to exist

Started as a proposed rework of `ginkgo-fd` once native `-fd` output landed
upstream in Ginkgo itself. The framing changed partway through: rather than
reworking a Ginkgo-dependent wrapper, build something that needs no BDD
framework at all. The key unlock: plain `go test -v` with `t.Run` subtests
already joins names with `/`, unambiguously, so the tree can be rebuilt with
zero disambiguation heuristics -- a smaller problem than `xctidy` solved,
and one that drops the BDD-framework dependency entirely (no separately
installed binary, no round-tripped JSON report).

## Architecture

- `parse.go` -- `Parse(io.Reader) ([]PackageResult, error)`. Buffers each
  package's raw `--- PASS/FAIL/SKIP` lines and, once the package boundary
  (the `ok`/`FAIL` summary line) is reached, filters to leaves only:
  Go prints a result line for *every* `t.Run` at *every* depth (a parent's
  own line rolls up its children's outcome once they've all finished),
  unlike RSpec/Ginkgo where only `it` blocks produce a line -- so
  `leavesOnly` drops any result whose name is a strict prefix of another
  result's name. Captured `t.Log`/`t.Error` output is bound to the *next*
  result line seen, matching `-v` mode's actual ordering.
- `render.go` -- `Render([]PackageResult, Style, io.Writer, bool)`. Same
  dedupe-shared-prefix tree walk as `xctidy`'s `Engine`, walked over
  `/`-split hierarchy instead of comma-disambiguated names. Three styles
  (`StyleClassic`/`StyleFd`/`StyleFs`) share one closing footer.
- `main.go` -- flag parsing (`-fd`/`-fs`/`--format documentation|spec`,
  default classic) plus `openInput`, which picks between two modes: read
  piped stdin directly, or shell out to `go test -v <args>` itself. Both
  `go test -v ./... | gorderly -fd` and `gorderly -fd .` work.

## Deliberately not built (v1)

- **No dots/progress mode.** `gotestsum` already owns that format well;
  no reason to duplicate it.
- **No JSON-report input mode.** No Ginkgo dependency to produce one.
  `go test -json` could be a more robust future input format, not built
  for v1.

## Status

- **Naming**: settled on `gorderly` after checking real prior art each
  round (`gotidy`, `gotree`, `goneat`, `gospec`, `behave`, `gobdd`, `goren`
  all ruled out for real collisions). Clean across GitHub and `pkg.go.dev`.
- **CI**: `.github/workflows/ci.yml` -- checkout, `setup-go`, `go build`,
  `go test -v ./...`. No lint step, matching every other Go/Swift repo in
  this account.
- **Released**: `v0.1.0` tagged and published; `go run
  github.com/woodie/gorderly@latest` confirmed resolving and running for
  real from an unrelated directory.
- **Own tests**: migrated off hand-rolled `t.Run` onto `sclevine/spec`
  (`parse_test.go`/`render_test.go`, `spec.Run(t, "...", func(t
  *testing.T, when spec.G, it spec.S) {...})`). All 23 specs pass via
  `go test -v ./... | gorderly -fd` (gorderly dogfooding its own binary on
  its own suite).

## Why `spec`, not Ginkgo/Gomega, for gorderly's own tests

Ginkgo doesn't route through `t.Run` at all -- it owns its own execution
and reporting -- so if gorderly's own tests adopted real Ginkgo/Gomega,
`go test -v` would show one flat wrapper test with nothing for gorderly's
own parser to build a tree from. That would silently break gorderly's
self-dogfooding (`make test` -> `go run . -fd ./...`). `sclevine/spec`
(verified by reading `parser.go`, not just trusting the README: every leaf
spec runs through a real `t.Run`) has no such conflict.

The fuller exploration -- language-mechanism sketches considered before
finding `spec`, the Swift/Kotlin `GoodTimesSpec` comparison that shaped the
design goals, the case for adopting `spec` over hand-rolling -- lives in
git history for this file and in `~/workspace/spec`'s own `docs/COWORK.md`,
which is now the canonical home for anything about `spec` itself (including
the fork's own additions: `Context()`/`T()`/`Describe`/`Var[T]`/
`RunAliased`). Not repeated here to avoid two copies of the same history
drifting apart.

## Next: migrate to `expect`

`parse_test.go`/`render_test.go` use `spec` for structure but still assert
with plain `if x != y { t.Error(...) }` -- `spec` deliberately ships no
assertions of its own. `~/workspace/expect` (Gomega-style, generics-based,
built alongside `lambada`'s own Ginkgo/Gomega migration) is the intended
replacement; `lambada`'s test suite already made this move, `gorderly`'s
hasn't yet. Open item for a future session.

## Sandbox limitation

No Go toolchain here (same situation as `humane`, `lambada`, `spec`,
`expect`) -- all Go changes are written by inspection and verified on the
user's own Mac. Needs, on your Mac:

```
cd ~/workspace/gorderly
go mod tidy
go test ./...
make check
go run . -fd .
```
