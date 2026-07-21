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
this file's own git history. (An earlier version of this note pointed at
`~/workspace/spec`'s own `docs/COWORK.md` as the canonical home for the
fork's additions -- that file no longer exists: `spec`'s `master` was
reset to plain `upstream/master` once every real consumer walked back off
the fork, wiping the fork-only `docs/` folder along with it. See
"Reversal: moved off the `woodie/spec` fork" below for why.)

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

## Reversal: moved off the `woodie/spec` fork, back to plain upstream

`parse_test.go`/`render_test.go` (and this README's "full toolchain"
example) used `spec.RunAliased` against the `github.com/woodie/spec` fork.
Walked back in favor of plain `github.com/sclevine/spec` -- no `replace`
directive, no `RunAliased`, no `.AsContext()`. The fork's whole value
turned out to be self-inflicted: `Aliases`/`RunAliased` exist to hand
`before`/`after`/`context` in as bound parameters so no per-file alias line
is needed, but the one-line alternative -- `context, before, after :=
describe, it.Before, it.After`, written by hand at the top of the suite
function -- needs nothing from the fork at all, and reads more plainly
than a six-parameter suite-function signature does. `it.T()`/`it.Context()`
turn out to be unnecessary too: since `spec` re-evaluates the whole suite
function per spec, the function's own `t *testing.T` parameter is already
correctly scoped inside any `before`/`after`/`it` closure by ordinary Go
closure capture (confirmed against `spec`'s own `t_test.go`, which could
have written `t.TempDir()` directly instead of `it.T().TempDir()` the
whole time). `go.mod` now depends on `github.com/sclevine/spec v1.4.0`
directly, real upstream, no `replace` line -- confirmed clean on a real
Mac: `go mod tidy` resolved without a stale checksum, and `go test -v
./...` (piped through `gorderly` itself) passes all 29 specs. `expect` is
unaffected -- kept as-is, dot-imported, paired with plain upstream `spec`
rather than the fork.

Same session: `expect_alias_test.go` renamed to `config_test.go`, which
now carries two things -- the real `expect` alias (actual code, runs as
part of the suite) and a commented-out `/* ... */` translation of a
familiar RSpec `Calculator` example into `describe`/`context`/`it`/
`before`/`after`, kept purely as a non-executing visual reference. This
file is a deliberate exception to the account's usual one-line-comment
rule (`~/workspace/woodie/docs/COWORK.md`, "Comments"): its whole purpose
is to be documentation as much as code, so the example is allowed to be a
real, multi-line block rather than pushed out to a companion doc.

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

## Classic style: partial coloring on the passing leaf

The default (`StyleClassic`) passing-leaf line used to be colored as one
solid green string (`colorize(green, renderLeaf(...))` in `colorizePass`),
covering the glyph, the name, and the `(N seconds)` timing all at once. The
user wanted only the glyph green and, inside the timing parenthetical, only
the number green -- name and the literal `(`/`seconds)` text left in the
terminal's default (effectively black) color, matching a real xctidy
terminal screenshot shown as reference. `colorizePass` gained an explicit
`StyleClassic` case building the line piecewise (`colorize(green, "✔") + "
" + name + " (" + colorize(green, formatSeconds(r.Elapsed)) + " seconds)"`)
instead of falling through to the old single-color default branch, which
still handles `StyleFd` unchanged. Only the passing case changed -- failing
lines stay entirely red and skipped lines stay entirely cyan, since
`colorizePass` is only ever called from `Render`'s `default:` switch arm
(the non-fail, non-skip case). Made by inspection only, same
no-Go-toolchain limitation as every other round above -- needs `go test
./...` and a real `go run . .` (or `make check`) on the user's Mac to
confirm the ANSI codes land as expected and no existing color-disabled
test (`render_test.go`'s "color is disabled" context) regresses.

## Full xctidy/gorderly parity sweep (v0.3.0)

After the classic partial-coloring fix above was confirmed on the user's
Mac (`go test ./...`, all 29 specs; `go run . .` dogfooding), the user asked
for a broader parity check against `xctidy`'s `Engine.swift` -- the two
tools are meant to be the same RSpec-flavored experience ported to two
languages (see `~/workspace/woodie/docs/COWORK.md`'s "Goal" section), so a
divergence in one is a bug in the other unless there's a stated reason
(`-fv`'s missing `Test Files` line is the one deliberate, documented
exception -- see "Later session: `-fv`, ported from `gorderly`" above).
Comparing `labelForPassed`/`labelForSkipped`/`labelForFailed` in
`Engine.swift` against `render.go`'s old `renderLeaf`/`colorizePass` line by
line (plus `xctidy`'s `EngineSpec.swift`/`VersionFlagSpec.swift` against
`render_test.go`, which had no equivalent assertions) turned up six real
gaps, all fixed this round:

1. **Classic style was missing the `(FAILED - N)` cross-reference
   entirely** -- `renderLeaf`'s default case ignored the `n` parameter for
   failed leaves, so a failing classic line had no pointer to its numbered
   entry in the `Failures:` section below. `xctidy`'s classic renders
   `✖ foo (FAILED - 1) (0.0019 seconds)` (`EngineSpec.swift` line 136);
   gorderly's classic now matches.
2. **Classic fail/skip lines colored the whole line as one block** instead
   of the glyph-plus-time-number-only pattern the passing-line fix above
   already established. Fixed by giving `colorizeFail`/`colorizeSkip` (new
   functions, replacing the old shared `renderLeaf` + outer-`colorize`-wrap
   shape) the same partial-coloring treatment for `StyleClassic`.
3. **`-fd`'s pending color was cyan; `xctidy`'s `.doc` uses yellow**
   (`colorize(.yellow, ...)` in `labelForSkipped`, real RSpec `-fd`
   convention). Added a `yellow = "33"` const and switched `-fd`'s skip
   case to it.
4. **`-fs` was missing `(FAILED - N)`/`(SKIPPED)` text and used different
   glyphs** than `xctidy`'s `.spec` style: fail glyph `✖` instead of `✗`, no
   inline failure number; skip glyph `○` instead of `-`, no `(SKIPPED)`
   text. Both now match `xctidy` exactly (`✗ foo (FAILED - 1)`,
   `- foo (SKIPPED)`).
5. **No `--version`/`-v` flag at all.** `xctidy` has `wantsVersion`
   (`VersionFlagSpec.swift`), checked before reading stdin so a bare
   `--version` doesn't hang. Ported the same check into a new `version.go`
   (`wantsVersion` + `gorderlyVersion` const), wired into `main.go`'s `run`
   before `parseFlags`.
6. **No TTY autodetection for color** -- `xctidy`'s `main.swift` checks
   `isatty(fileno(stdout))`; gorderly only checked `NO_COLOR`, so it would
   emit raw ANSI codes into redirected/non-terminal output where `xctidy`
   wouldn't. Added `isTerminal` in `main.go`, reusing the same
   `os.ModeCharDevice` check `openInput` already applies to stdin, combined
   with the existing `NO_COLOR` check.

One deliberate difference from `xctidy`, not a gap: `gorderlyVersion` in
`version.go` is a hand-bumped constant, not derived from `git describe` at
build time like `xctidy`'s `Version.swift`. `gorderly`'s README documents
`go install github.com/woodie/gorderly@latest` as the primary install path
-- a module-proxy fetch with no `.git` metadata to describe -- so the
version has to already be the right string in the committed source at the
tagged commit; a `make version`-style Makefile target (which only helps a
`git clone && make build` flow) wouldn't cover that path. Noted in the
README's new "Cutting a release" note under Development.

`render.go`'s old `renderLeaf` function (one shared shape wrapped in a
single outer `colorize` call per state) was deleted outright rather than
kept alongside the new `colorizePass`/`colorizeFail`/`colorizeSkip` --
matching `xctidy`'s own per-state function split
(`labelForPassed`/`labelForSkipped`/`labelForFailed`), since the partial
vs. whole-line coloring difference between styles can't be expressed by
one shared string-building helper with a generic color wrap.

Added test coverage for all six fixes to `render_test.go` (new "classic
style with color enabled" context asserting partial coloring on all three
states; new assertions in the existing "in fd style"/"in fs style"
contexts for the corrected colors/glyphs/text), plus new `version_test.go`
(`wantsVersion`, mirroring `VersionFlagSpec.swift`'s cases) and
`main_test.go` (`run`'s `--version`/`-v` handling -- new, since Go's `run`
function is actually unit-testable unlike Swift's top-level `main.swift`
script code, which is why `xctidy` has no equivalent).

Made by inspection only, same no-Go-toolchain limitation as every prior
round -- needs on the user's Mac:

```
cd ~/workspace/gorderly
go test ./...
make check
go run . --version
go run . . | cat        # confirm color auto-disables when stdout isn't a TTY
go run . .               # confirm color still shows in a real terminal
```

## v0.3.0 → v0.3.1: an `errcheck` failure, tagged before `make check` ran

The tag/push handoff above went out (`v0.3.0`, `fdbd425`) before `make
check` had actually been run -- Cowork's own no-Go-toolchain limitation
means the sandbox can never confirm this itself, but the tag/push
instructions still should have waited on the user's real `make check`
result first rather than going out immediately after committing. The user
ran it and hit a real `errcheck` violation: the new `--version` handler
(`fmt.Fprintln(stdout, gorderlyVersion)`) didn't check its return value,
unlike every other write in `main.go` (`warn`'s `_, _ = fmt.Fprintf(...)`).

Fixed with `_, _ = fmt.Fprintln(...)`, matching the existing convention.
Since `v0.3.0`/`fdbd425` had *already been pushed* to `origin/main` by the
user in between the tag/push handoff and the lint report (not stated
explicitly at the time, only inferred afterward from `git fetch` +
`git log --oneline origin/main`), amending that commit locally created a
real fork: local `main` became a sibling of `origin/main`, not a
descendant, so the next push was rejected as non-fast-forward, and
re-tagging `v0.3.0` to point at the amended commit collided with the tag
already on the remote ("already exists"). Recovered by `git reset --hard
origin/main` (discarding the amend), reapplying the one-line fix as a
*new* forward commit, and tagging that as `v0.3.1` instead of trying to
force anything -- `main` then fast-forwarded cleanly and the push
succeeded, confirmed via `git merge-base --is-ancestor origin/main HEAD`
before pushing. Local `v0.3.0` was also re-pointed back at `fdbd425` to
match what's actually on GitHub (verified directly via
`github.com/woodie/gorderly/releases/tag/v0.3.0`, not just local git
state), rather than left dangling at the abandoned amended commit.

Lesson generalized into `~/workspace/woodie/docs/COWORK.md`'s "Tagging
releases" section: run `check`, confirm clean, *then* tag, *then* tell the
user to push -- and never amend a commit once there's any chance it's
already reached the remote; make a new forward commit instead.

`docs/releases/v0.3.1.md` covers the errcheck fix on its own; the `gh
release create` handoff for GitHub's actual Releases feature (as opposed
to the tag pages GitHub renders automatically from annotated-tag messages)
consolidates `v0.3.0` + `v0.3.1`'s notes into one `v0.3.1` release, at the
user's choice, since `v0.3.0` was superseded within the same session and
never really shipped to anyone as a standalone release.

Tagged `v0.3.0` after the user confirmed. See `docs/releases/v0.3.0.md`
for the release notes.
