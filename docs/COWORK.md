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
  `/`-split hierarchy instead of comma-disambiguated names. Four styles
  (`StyleClassic`/`StyleFd`/`StyleFs`/`StyleFv`) -- the first three share
  one closing footer; `StyleFv` closes with Vitest's own `Test
  Files`/`Tests`/`Duration` shape instead (see below).
- `main.go` -- flag parsing (`-fd`/`-fs`/`-fv`/`--format
  documentation|spec|vitest`, default classic) plus `openInput`, which
  picks between two modes: read piped stdin directly, or shell out to
  `go test -v <args>` itself. Both `go test -v ./... | gorderly -fd` and
  `gorderly -fd .` work.

### `-fv`: matching Vitest, not wrapping it

Added so a Go monorepo running both `vitest run` and `go test` side by
side (e.g. `lambada`) gets visually consistent output without gorderly
ever touching or reformatting Vitest's own output -- Vitest's real
reporters (confirmed from its source, not guessed: `packages/vitest/src/
node/reporters/renderers/{utils,figures}.ts`) use `✓`/`×`/`↓` (not
`✔`/`✖`/`⊘`, and the fail glyph is a multiplication sign, not `✗`), a
two-toned green duration (`colorize green` for the number, a separate
`brightGreen`/92 for the `ms`/`s` unit), and a footer with labels
right-justified to 11 columns (`padSummaryTitle`'s `str.padStart(11)`).
`formatVitestDuration`/`formatVitestDurationParts` reproduce Vitest's own
`formatTime`: whole ms under 1000ms, seconds to two decimals at or above.
One real, unavoidable precision gap: `go test -v` only reports elapsed
time to two decimal places of a second, so fast subtests routinely show a
flat `0ms` where Vitest's own finer JS timers would show `2ms`/`4ms` --
documented in the README's Limitations section, not something `-fv` can
fix. `gotestsum` already owns JUnit/dots/progress formats well (see
"Deliberately not built" below); `-fv` isn't competing with it, it's
matching a JS test runner's own terminal conventions instead.

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
  real from an unrelated directory. `v0.2.0` adds `-fv`/`--format vitest`.
- **Own tests**: migrated off hand-rolled `t.Run` onto `sclevine/spec`,
  later onto the `woodie/spec` fork + `expect` (`spec.RunAliased` +
  `parseSuite`/`renderSuite` named functions, see "Done: migrated to
  expect" below for the current shape). All 29 specs pass via `go test -v
  ./... | gorderly -fd` (gorderly dogfooding its own binary on its own
  suite) -- confirmed on the user's own Mac, including the new `-fv`
  cases.

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

## Done: migrated to `expect`, full parity with `lambada`/`humane`

The open item above is closed. `parse_test.go`/`render_test.go` used to
assert with plain `if x != y { t.Fatalf(...)/t.Errorf(...) }` on bare
upstream `spec.Run`/`when`/`it.Before` -- no `expect`, no fork. Brought all
the way to the same shape `lambada` and `humane` are already on, not just
a partial assertion swap:

- `go.mod` now requires `github.com/woodie/expect v0.2.0` and replaces
  `github.com/sclevine/spec` with `github.com/woodie/spec v0.1.0` (was
  plain upstream `v1.4.0`, no fork, since nothing here needed the fork's
  additions until now).
- `TestParse`/`TestRender` are one-liners into named `parseSuite`/
  `renderSuite` functions via `spec.RunAliased`, `when(...)` renamed to
  `context(...)` (these are scenarios, not feature groupings -- no
  `describe` wrapper needed at this nesting depth).
- Every assertion is `expect(got, t).To(Matcher)` via the recommended
  lowercase alias (new `expect_alias_test.go`, shared by both files),
  using `Succeed`/`Equal`/`DeepEqual`/`Contain`/`NotTo` in place of the old
  hand-rolled `if`/`t.Errorf` checks. Two spots (`pkg.Results[i].Hierarchy`,
  `r.Output`) use `DeepEqual` on the whole slice instead of the original
  `strings.Join(...)`-then-compare workaround -- a real small improvement,
  not just a mechanical port, since `DeepEqual` compares the slice
  directly and needed no string-join step to begin with.
- Two `it`s that originally combined two conditions with `||` into one
  `t.Fatalf`/`t.Errorf` (`records the package with no results`) needed a
  guard (`if len(pkgs) == 1 { ... }`) around the second `expect` call to
  preserve the original short-circuit safety -- splitting an `||` into two
  independent `expect` calls loses that guarantee, since Go doesn't
  short-circuit between separate statements the way `||` does within one.

No behavior change to `Parse`/`Render`/`gorderly`'s actual output -- test
suite and tooling only, so no new `gorderly` version tag, matching the
same call made for `lambada`'s and `humane`'s equivalent updates.
Confirmed for real on the user's own Mac: `go test -v ./... | gorderly
-fd` -- 0 failed, 0 skipped, 29 total, gorderly dogfooding its own binary
on its own newly-migrated suite.

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
go run . -fv .
```

Also no `git push` access -- confirmed the hard way this session (a
`git push origin --delete <branch>` in an unrelated repo failed with
"Host key verification failed"). Commits and annotated tags get made
locally in the sandbox (same mounted filesystem as the user's Mac), but
`git push origin main --tags` (or equivalent) is always handed off for
the user to run themselves.
