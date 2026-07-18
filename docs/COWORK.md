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

Started as a proposed rework of `ginkgo-fd` (see that repo's `docs/COWORK.md`
issue #1, now won't-fix) once native `-fd` output landed upstream in Ginkgo
itself ([onsi/ginkgo#1670](https://github.com/onsi/ginkgo/pull/1670), merged
`15b9f44`). Partway through scoping that rework, the framing changed: rather
than reworking a Ginkgo-dependent wrapper, build something that needs no
BDD framework at all. The key technical unlock: Ginkgo/Quick's problem was
that `Describe`/`Context`/`It` names get flattened into one comma-joined
string, which is why `xctidy` needs a whole comma-disambiguation dictionary
(see that repo's `docs/HOW_IT_WORKS.md`) and why `ginkgo-fd` needed
Ginkgo's own structured JSON report to recover the hierarchy at all. Plain
`go test -v` with `t.Run` subtests doesn't have that problem -- subtest
names are already joined with `/`, unambiguously, so the tree can be
rebuilt with zero disambiguation heuristics. That's a smaller problem than
the one `xctidy` solved, and it drops any BDD-framework dependency, which
was the actual friction point: `humane`'s `test` target (`ginkgo-fd -r`)
shells out to a separately-installed `ginkgo` binary and round-trips a temp
JSON file, versus `humane-swift`'s `test` target (`swift test 2>&1 |
xctidy`), a single pipe with nothing else installed. That difference --
observed directly by comparing the two Makefiles -- is what motivated
starting fresh instead of reworking `ginkgo-fd` in place.

## Naming

Landed on `gorderly` after a long process of elimination, checked against
real prior art each round rather than picked in isolation:

- `gotidy` -- taken, an active GitHub org (`github.com/gotidy`, 24 repos)
  plus two unrelated `GoTidy` projects.
- `gotree` -- very crowded, 6+ unrelated Go tree-printing/tree-manipulation
  projects.
- `goneat` -- collides with `yaricom/goNEAT`, an unrelated neuroevolution
  ML library.
- `gospec` -- taken, and taken badly: an actual (if long-unmaintained)
  Go BDD *testing framework* from ~2012 (`luontola/gospec`, several forks),
  same problem space, high confusion risk.
- `behave` -- Python's dominant BDD CLI tool (`pip install behave`), a
  direct installed-binary collision, disqualified outright.
- `gobdd` -- a real, actively-referenced Go BDD framework
  (`go-bdd/gobdd`), same language, same problem space.
- `goren` -- a real but obscure Linux CLI file-renaming tool
  (`mikelexp/goren`), soft collision (low adoption, different domain, but
  literally an installed `goren` binary).
- `yubari` -- clean except for one obscure, low-adoption FFXIV-adjacent Go
  package; considered, not chosen.
- `gospruce`/`gocanopy`/`gocopse` -- all clean, all playing on
  tidy-and-tree double meanings (mirroring `xctidy`'s own thesaurus-derived
  naming, and the fact that this tool renders a literal nested tree).
  Runners-up.
- **`gorderly`** -- clean across GitHub, `pkg.go.dev`, and general web
  search (only unrelated hits: a couple of dormant social handles, an
  unrelated home-organizing newsletter). No software collisions at all.
  Final answer.

## Architecture

- `parse.go` -- `Parse(io.Reader) ([]PackageResult, error)`. Buffers each
  package's raw `--- PASS/FAIL/SKIP` lines and, once the package boundary
  (the `ok`/`FAIL` summary line) is reached, filters to leaves only:
  Go prints a result line for *every* `t.Run` at *every* depth (a parent's
  own line rolls up its children's outcome once they've all finished),
  unlike RSpec/Ginkgo where only `it` blocks produce a line -- so
  `leavesOnly` drops any result whose name is a strict prefix of another
  result's name. Captured `t.Log`/`t.Error` output is bound to the *next*
  result line seen (`pending`), matching `-v` mode's actual ordering (log
  output interleaves during execution, before the closing `--- PASS/FAIL`
  line for that specific subtest).
- `render.go` -- `Render([]PackageResult, Style, io.Writer, bool)`. Same
  dedupe-shared-prefix tree walk as `xctidy`'s `Engine` and
  next-caltrain-kotlin's Gradle `TestListener`, just walked over `/`-split
  hierarchy instead of comma-disambiguated names. Three styles
  (`StyleClassic`/`StyleFd`/`StyleFs`) share one closing footer
  (`Test Succeeded`/`Test Failed` + `Tests Passed: X failed, Y skipped, Z
  total`) -- `xctidy` learned the hard way that stacking each style's own
  native summary on top of that footer reads as "confusing" in real
  side-by-side output (see its `docs/COWORK.md`, "footer un-stacking");
  `gorderly` starts from that resolved lesson instead of relearning it.
- `main.go` -- flag parsing (`-fd`/`-fs`/`--format documentation|spec`,
  default classic) plus `openInput`, which picks between two modes: read
  piped stdin directly when nothing is piped in AND no package path is
  given, the `xctidy`-style "raw output piped straight in" path; otherwise
  shell out to `go test -v <args>` itself, the `ginkgo-fd`-style wrapper
  convenience path. Both `go test -v ./... | gorderly -fd` and
  `gorderly -fd .` work.

## Deliberately not built (v1)

- **No dots/progress mode.** `gotestsum` already owns that format well
  (`--format dots`) and is the established, widely-adopted tool for it --
  no reason to duplicate it. `make check` here uses the same
  silent-on-success/dump-log-on-failure shell pattern
  `humane-swift`/next-caltrain-{kotlin,swift} already use, no tool
  cooperation needed.
- **No JSON-report input mode.** `ginkgo-fd` supported `ginkgo-fd
  report.json` as a direct-format path; `gorderly` has no equivalent since
  there's no Ginkgo dependency to produce one. `go test -json` exists and
  could be a more robust future input format (structured events instead of
  regex-parsed text, closer to `gotestsum`'s own approach) but wasn't
  built for v1 -- the `xctidy` parallel is piping raw text, and that's
  what shipped first.

## Sandbox limitation

No Go toolchain here (same situation as `humane`, `lambada`) -- `parse.go`,
`render.go`, and `main.go` were written by inspection, with `parse_test.go`/
`render_test.go` covering the parsing/rendering logic using stdlib nested
`t.Run` (the same hierarchical-test convention this account uses
everywhere, per `~/workspace/woodie/docs/COWORK.md`'s "Test structure"
section) rather than a BDD framework. **None of this has been run for
real.** Needs, on your Mac, in order:

```
cd ~/workspace/gorderly
go mod tidy
go test ./...
make check
go run . -fd .
```

If `go test -v` on a real project doesn't come out looking right, the most
likely culprits are in `parse.go`: the exact whitespace/indentation Go's
real `-v` output uses (regexes here are written from documented behavior,
not a captured real transcript), and whether `t.Log` output really does
interleave *before* the closing `--- PASS/FAIL` line in every Go version
(the `pending`-binds-to-next-result-line assumption in `Parse`). A single
real `go test -v` transcript pasted back would resolve both quickly.

## Confirmed working, first fixes

The user ran the sandbox-written code for real on their Mac. `go test ./...`
caught one real test bug on the first pass: `render_test.go`'s "prints the
shared context path once" case counted `"TestMath"`/`"addition"`
occurrences across the *entire* rendered output, but the `Failures:`
section legitimately reprints each failure's full hierarchy path (matching
`xctidy`/`ginkgo-fd`'s own convention) -- so the count was never going to
land on 1 once a failure existed. Fixed by scoping the count to the tree
portion only (`strings.Cut(out, "Failures:")`), not by changing the render
behavior, which was already correct. Also humanized the `Failures:`
section's hierarchy (it was printing raw underscore-joined names,
inconsistent with the tree above it) and fixed `golangci-lint`'s `errcheck`
complaints (five unchecked `fmt.Fprintf(stderr, ...)` calls in `main.go`
collapsed into one `warn()` helper; three unchecked `Render(...)` calls in
`render_test.go` given explicit `_, _ =`). All three render styles then
confirmed for real, one at a time, across three separate runs: `-fd`,
`-fs`, and classic (via `make check`) -- all clean, all 23 specs passing,
`gorderly` correctly formatting its own suite. `golangci-lint run` reports
zero issues. Committed as `d513f8e` ("First round.") and pushed.

## CI, badges, and the first release

Pulled `ginkgo-fd`'s CI/badge shape as the starting template per the user's
ask, but used `humane`'s `.github/workflows/ci.yml` instead once compared
side by side -- `humane`'s is the better fit for `gorderly` specifically,
since it doesn't need `ginkgo-fd`'s "install ginkgo CLI" step at all (no
framework dependency to install). `.github/workflows/ci.yml`: checkout,
`actions/setup-go@v4` pinned to `1.25.0` (matching `go.mod`'s `go`
directive, same comment `humane`'s workflow uses to explain the pin),
`go build -v ./...`, `go test -v ./...`. No `golangci-lint` step in CI --
checked across every Go repo in this account (`humane`, `zouk`'s Swift CI,
`homebrew-zouk`) and none of them wire lint into CI either, so `gorderly`
doesn't either, matching established precedent rather than improvising one.

`README.md` badge row matches `humane`'s/`ginkgo-fd`'s exact four badges
(go.mod version, CI, Release, License), same shields.io URL shapes with
`gorderly` substituted in. `LICENSE` is MIT (from `gh repo create`'s
default), confirmed before adding the License badge rather than assumed.

`docs/releases/v0.1.0.md` added, matching the `docs/releases/vX.Y.Z.md`
per-release-notes convention `humane`/`humane-swift` both use (not
`humane`'s own `docs/COWORK.md`, which is the session log, not release
notes -- the two are kept separate in every sibling repo). Tagging/pushing/
`gh release create` all need to happen on the user's own machine (no
network route to GitHub from this sandbox -- see
`~/workspace/woodie/docs/COWORK.md`'s "Pushing" section); the exact
command, matching `humane-swift`'s own documented pattern:

```
git tag v0.1.0
git push origin v0.1.0
gh release create v0.1.0 --title "v0.1.0" --notes-file docs/releases/v0.1.0.md
```

Confirmed run for real later the same session -- a `go run github.com/woodie/gorderly@latest` invocation from an unrelated directory printed `go: downloading github.com/woodie/gorderly v0.1.0`, proving the tag/push/release sequence above was actually completed on the user's Mac, not just handed off.

## Test-writing convention: evaluating `ginkgo-fd`'s and `lambada`'s Ginkgo/Gomega style vs. something native to `gorderly`

Prompted by asking `gorderly`'s own tests to match the `Describe`/`Context`/`It`
style already used elsewhere in this account (`zouk`/`next-caltrain-swift`'s
Quick specs, `next-caltrain-kotlin`'s Kotest specs, and -- it turned out --
`ginkgo-fd`'s own `main_test.go`, which really does use real Ginkgo/Gomega).
That surfaced a real conflict worth stating plainly: Ginkgo doesn't route
through `t.Run` at all -- it owns its own execution/reporting -- so if
`gorderly`'s own tests adopted real Ginkgo/Gomega, `go test -v` would show one
flat wrapper test with nothing for `gorderly`'s own parser to build a tree
from. Adopting Ginkgo for `gorderly`'s tests would silently break
`gorderly`'s own self-dogfooding (`make test` -> `go run . -fd ./...`).

Real scope check (not assumed): grepped every Go `go.mod` in this account.
`humane` and `lambada` both depend on `onsi/ginkgo`/`onsi/gomega` for real;
`ginkgo-fd`'s own suite does too (ironic, given the tool it ships). The
`ginkgo` fork obviously has to keep testing itself with Ginkgo regardless of
any of this.

### Design goals, written down before touching any code

- `describe`/`context`/`it` nesting built on literal `t.Run`, so `go test
  -v`/`gorderly`/every IDE's "run test" button all work on it natively.
- Set up context above, plug into it below -- an `it` should never derive its
  own scenario, matching `~/workspace/woodie/docs/COWORK.md`'s existing "Test
  structure" rule, not a new one invented here.
- Guaranteed-fresh state per `it`, matching what Quick/Kotest give for free.
- Zero required third-party dependencies (`testing` + `strings`; generics
  don't need an import, so still in-bounds if useful).
- Reads close to the Swift/Kotlin specs already in the account.
- Adding a new `it` to an existing `context` is a pure addition.

Non-goals: not cloning Ginkgo's full surface (no `FIt`/`XIt`, no parallel
spec execution model, no general matcher library); not hiding `t.Run` behind
a competing abstraction.

### `GoodTimesSpec.swift`/`GoodTimesSpec.kt` comparison (the `debugOverrideDotw` context)

Read both real files (`next-caltrain-swift/Tests/GoodTimesSpec.swift:150`,
`next-caltrain-kotlin/.../GoodTimesSpec.kt:152`) side by side. Same shape,
two different solutions, because the two frameworks offer different hooks:

- **Swift/Quick** uses `justBeforeEach` -- guaranteed to run after every
  nested `beforeEach`, right before the `it`. The outer context owns the
  shared "how" (override + rebuild), inner contexts each set just the
  varying "what" via their own `beforeEach`.
- **Kotlin/Kotest** has no `justBeforeEach` at all, so it fuses "what" and
  "how" into one named function (`setDotw(dotw: Int)`), called directly by
  each inner context's `beforeEach`.

Go has no lifecycle hooks whatsoever, so it's structurally closer to
Kotest's constraint than Quick's -- the named-function shape is the one to
port, not the split-hook shape. One Go-specific rule this forces: the
factory call has to live *inside* each `it`, not at the context level,
since nothing reruns a parent closure per child the way Kotest/Quick do
automatically. `t.Cleanup` is stdlib's own real primitive for the
`afterEach` half -- scoped, LIFO, panic-safe -- not something to hand-roll
with `defer` at the wrong scope.

### Language-mechanism exploration (evaluated, ultimately superseded)

Sketched three ways to make hand-rolled `t.Run` read like `describe`/
`context`/`it`, at the user's explicit request to compare package-level
function variables, wrapper functions, and function type aliases:

- `var describe = (*testing.T).Run` / `var context = (*testing.T).Run` --
  Go method expressions turn `t.Run` itself into a renamed function value,
  zero new mechanism. (Precedent: Ginkgo's own `Context` is a bare alias for
  `Describe` internally, for the same readability reason.)
- `func it(t *testing.T, name string, fn func(*testing.T)) bool` -- a thin
  wrapper, not a bare alias, specifically to auto-prefix `"it "` once
  instead of every call site retyping it.
- A generic `Setup[T any]`/`itUsing(...)` pairing was drafted and rejected --
  more ceremony at the call site than just calling a plain closure as the
  first line inside `it`.

This whole approach was superseded once `sclevine/spec` was found (below) --
kept here because the mechanics (method expressions, the `t.Cleanup`
finding, the Kotest-shaped factory rule) are still exactly right and may be
useful if `spec` is ever dropped for some reason.

### Finding and verifying `sclevine/spec`

The user's ask -- "if there is any existing library we can use for BDD
style but within the standard test framework, we should use it" -- led to
`github.com/sclevine/spec`. Verified by actually reading its source
(`parser.go`), not trusting the README: every leaf spec runs through a real
`t.Run(name, func(t *testing.T) {...})` call. Its own doc comments claim it
re-evaluates the whole `when`/`it` tree fresh for every single spec ("does
not reuse any closures between test runs, to avoid test pollution") --
`it.Before`/`it.After` are real hooks, not a convention someone has to
remember, which directly answers the `t.Cleanup`-is-on-the-developer
concern raised about the hand-rolled approach above.

Two honest caveats surfaced before recommending it: last tagged release
(`v1.4.0`) was December 2019 -- stale, though the core mechanism only
depends on Go subtests, stable since Go 1.7, so lower-risk than it sounds.
And it ships more surface than the stated non-goals wanted (`Focus`/`Pend`/
`Random`/`Reverse`/`Parallel`/`Global`/`Flat`/`Nested`) -- not a cost if
unused, but not as minimal as the hand-rolled sketch either.

**Verified for real, twice, not just read from source:**

1. `~/workspace/spec` -- the user's own clone of the library. `go test -v
   ./...` run for real: the library's entire own suite passes, including
   `TestSpec`, its full quickstart-equivalent demo (`Before`/`After`,
   `Random`/`Reverse`/`Parallel` ordering, `Focus`/`Pend`, a custom
   `report.Terminal{}` reporter) -- confirmed the reporter is opt-in and
   doesn't affect output when unused (every other test in the suite prints
   plain `go test -v` text with no custom banner).
2. `~/workspace/spec-demo` -- a fresh module, linked to the local `../spec`
   clone via `go.work` (same no-network-fetch pattern `ginkgo-fd`'s own
   `docs/COWORK.md` already documents for linking a local `ginkgo` fork).
   `goodtimes_test.go` translates the `debugOverrideDotw` Friday/Saturday/
   Sunday scenario from the Swift/Kotlin comparison above into `spec`'s
   `when`/`it.Before` shape (the Kotest-style fused function, per the rule
   established above -- `spec` only has `Before`/`After`, no
   `justBeforeEach`, so the same ordering constraint applies). Also added a
   deliberate stress test: mutate state in one `it`, assert a sibling `it`
   doesn't inherit the mutation -- a real empirical check of `spec`'s
   freshness claim, not trust-the-docs. `go test -v ./...` run for real:
   all 5 specs pass, freshness stress test included.

**Then the actual end-to-end check**: piped `spec-demo`'s real `go test -v`
output through `gorderly -fd` (`go run github.com/woodie/gorderly@latest
-fd`, resolving the just-tagged `v0.1.0`). Rendered as a correct, fully
deduped nested tree, single shared footer, byte-for-byte what was designed
for. Along the way, read `parser.go` closely enough to explain *why* this
works even though `spec`'s default mode is flat, not nested: intermediate
`when` groups don't get their own `t.Run` call unless `spec.Nested()` is
passed -- only the leaf `it` calls `t.Run`, with the entire ancestor path
pre-joined into one name string. `gorderly`'s `leavesOnly` filter (built
for the opposite case, where every level gets its own result line) is a
no-op-safe pass-through here, since there's nothing to filter -- both
`spec`'s flat default and its opt-in `Nested()` mode render correctly
through `gorderly`, by different but equally correct paths through
`gorderly`'s own parser.

Net conclusion: adopt `sclevine/spec` for the structural layer (`when`/`it`/
`Before`/`After`) rather than the hand-rolled aliases above. Still need our
own small `assertEqual`/`assertContains`-style helpers, since `spec`
deliberately ships no assertions of its own.

### README rework

`README.md` gained a "Writing tests with `spec`" section (the `GoodTimes`
Friday snippet from `spec-demo`, trimmed) between "Usage" and the
gotestsum/Ginkgo comparisons, which were both cut down to one line each per
the user's request -- the "Why not gotestsum?" heading was dropped entirely
and folded into the intro paragraph; the Ginkgo limitations bullet lost its
`onsi/ginkgo#1670` citation. `## Known limitations` shortened to `##
Limitations`. Not yet done: `parse_test.go`/`render_test.go` still use plain
hand-rolled `t.Run` with no `spec` -- the README's `spec` section describes
the recommended pattern, not `gorderly`'s own current test files yet. That
migration is the natural next session's starting point.

## Migrating `parse_test.go`/`render_test.go` to `spec`

Done in the next session. `go.mod` gained `require
github.com/sclevine/spec v1.4.0` (the latest tag on the real repo -- checked
via `git tag` against the user's own `~/workspace/spec` clone rather than
guessed). Both test files were rewritten to `spec.Run(t, "...", func(t
*testing.T, when spec.G, it spec.S) {...})`, keeping every existing
assertion and case, restructured as: shared setup lives once in `it.Before`
at the top of each `when` block, each `it` only asserts -- never derives its
own setup, per the account's standing test-structure rule. Test/case names
had their leading `given`/`when`/`it` words stripped where redundant with
the DSL call itself (`when("given a transcript mixing...")` became
`when("a transcript mixes...")`, `it("it returns no error")` became
`it("returns no error")`) to match the phrasing already used in the
README's own `spec` sample.

Verification note: this sandbox has no Go toolchain at all (not just no
network -- `go` itself isn't installed here), so unlike every prior
verification round this session, this migration could not be compiled or
run locally before handing it back. A `go.work` linking to the user's local
`../spec` checkout was drafted to attempt it anyway, then deleted (via
`allow_cowork_file_delete`, since files placed in the workspace mount can't
be removed outright) once `go` turned out to be missing entirely -- so no
stray `go.work` was left in the repo.

Two things need to happen on the user's machine, which has both `go` and
network access, before this is trustworthy:

```
go mod tidy   # fetches sclevine/spec@v1.4.0 for real and writes go.sum
make check    # confirms parse_test.go/render_test.go actually compile and pass
```

Until `go mod tidy` runs for real, `go.sum` has no entry for `spec`, so
`go build`/`go test` will fail outside of a `go.work`-linked environment.
This is a real, not-yet-verified change, consistent with the account's
convention of docs describing actual state -- it works on paper and
against the source read of `spec`'s `parser.go` earlier this account's
history, but hasn't yet been run for real the way every earlier `gorderly`
change in this file was.

**Verified for real minutes later**: `go build -o gorderly && mv gorderly
~/go/bin/`, then `go test -v ./... | gorderly -fd`, dogfooding the
freshly-built binary on the migration itself. All 23 specs across both
files pass, rendered as a correct nested tree with a single shared footer
(`Tests Passed: 0 failed, 0 skipped, 23 total`) -- `go mod tidy` resolved
`sclevine/spec v1.4.0` cleanly and `spec`'s flat `t.Run` naming came through
`gorderly`'s parser exactly as designed. Migration closed out.
