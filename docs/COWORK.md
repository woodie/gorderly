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
