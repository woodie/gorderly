# Working with gorderly

Cross-project conventions (git locks, sandbox toolchain, pushing, comments, code style)
are in `~/workspace/woodie/docs/COWORK.md`.

## What this is

A Go equivalent of `xctidy`/`kotidy`, for plain `go test`. Reads `go test -v`'s raw
stdout (piped in, or by shelling out to `go test -v` itself when given a package
path) and re-renders it as a nested RSpec `-fd`-style tree, using the `/`-joined
hierarchy `t.Run` subtests already carry in their names. No BDD framework, no
`--json-report` file, no wrapped test-runner binary.

## Architecture

- `parse.go` -- `Parse(io.Reader) ([]PackageResult, error)`. Buffers each package's
  raw `--- PASS/FAIL/SKIP` lines and filters to leaves only (`leavesOnly`): Go prints
  a result line for every `t.Run` at every depth, not just leaves, so any name that's
  a strict prefix of another result's name gets dropped.
- `render.go` -- `Render([]PackageResult, Style, io.Writer, bool)`. Dedupes the shared
  hierarchy path against the previous leaf, walked over `/`-split segments. Four
  styles (`StyleClassic`/`StyleFd`/`StyleFs`/`StyleFv`) share one closing footer,
  except `StyleFv` which closes with Vitest's own `Test Files`/`Tests`/`Duration`
  shape.
- `main.go` -- flag parsing (`-fd`/`-fs`/`-fv`/`--format documentation|spec|vitest`,
  default classic) plus `openInput`, which reads piped stdin or shells out to
  `go test -v <args>`.

## Design decisions worth knowing

- **No `replace` directive for `woodie/spec`.** `go.mod` depends on plain upstream
  `github.com/sclevine/spec v1.4.0`, not the fork. `go install pkg@version` treats
  the target as the main module and unconditionally rejects any `replace` directive
  under that treatment -- confirmed the hard way in `v0.4.0`/`v0.4.1` (`go install
  github.com/woodie/gorderly@latest` failed outright even though the fork was only
  ever used in `_test.go` files). Don't reintroduce the fork here without solving
  that first. Libraries like `humane` don't have this problem since nothing runs
  `go install` against them.
- **`-fv` matches Vitest's real reporter source**, not a guess from its docs:
  `✓`/`×`/`↓` glyphs (fail glyph is a multiplication sign, not `✗`), a two-toned
  green duration, footer labels right-justified to 11 columns. `go test -v` only
  reports elapsed time to two decimal places of a second, so fast subtests show a
  flat `0ms` where Vitest's own finer JS timers would show `2ms`/`4ms` -- a real
  precision ceiling, not a bug.
- **Classic style colors only the glyph and the elapsed-time number**, not the whole
  line -- matches `xctidy`'s own partial-coloring convention. Fail/skip lines follow
  the same partial pattern.
- **The package import path prints as a plain label, not a tree node** -- it costs no
  indent level, and every top-level suite (including the first) gets a blank line
  before it to separate it from the label.
- **Deliberately not built**: dots/progress mode (`gotestsum` already owns that well)
  and a `go test -json` input mode (not needed while piped text works cleanly).

## Testing

`sclevine/spec` (plain upstream, no fork) + `github.com/woodie/expect`, matching
`lambada`/`humane`. `go test -v ./... | gorderly -fd` to see the real nested tree.
`make test`/`make check` wrap this.

## Sandbox limitation

No Go toolchain here -- all changes are written by inspection, verified via
`go mod tidy`/`go test ./...`/`make check` on the user's own Mac. Also no `git push`
access (SSH host key verification fails from the sandbox) -- commits and tags get
made locally, `git push`/`git push --tags` always hand off to the user.

## Current status

`v0.4.2`, CI green. Output parity-audited against `xctidy` in both directions
(classic/fd/fs/fv glyphs, colors, and footer shape match exactly).
