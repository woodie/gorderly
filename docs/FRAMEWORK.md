# Writing tests: `spec` + `expect`

How we structure Go tests -- structural functions and lifecycle hooks
from [`woodie/spec`](https://github.com/woodie/spec) (a fork of
[`sclevine/spec`](https://github.com/sclevine/spec); see that fork's own
README for why it exists), matchers from
[`expect`](https://github.com/woodie/expect), and `gorderly` rendering
whatever `go test -v` prints as a real tree. Each piece is independent --
`spec` needs no assertion library, `expect` needs no BDD framework,
`gorderly` only ever needs `go test -v`'s raw text -- but they're built to
be used together, and this doc shows what that looks like in real suites.
Examples below use a generic domain (`FileSize`, `Object`, an HTTP
middleware) rather than any one project's real code, so the pattern reads
the same regardless of what you're actually testing. The Swift side of
this pairing (`xctidy`) follows the same shape with different tools -- see
[`xctidy`'s own docs/FRAMEWORK.md](https://github.com/woodie/xctidy/blob/main/docs/FRAMEWORK.md)
if you're working on that side instead. For `expect`'s full matcher list,
see [its README](https://github.com/woodie/expect#readme).

## Why `spec`, not Ginkgo

Ginkgo doesn't route through `go test`'s own `t.Run` -- it owns its own
execution and reporting, so a suite written against it shows up as one
flat wrapper test under `go test -v`, with no real subtest tree for
anything (`gorderly` included) to parse. `spec` is built entirely on
`t.Run`: every `describe`/`context`/`it` is a real subtest, so its output
is exactly what any other `go test -v` consumer already understands, no
separate reporter or JSON round-trip required. That's also *why*
`gorderly` exists as a plain-text renderer rather than a Ginkgo-only tool
-- it works on any suite built on stdlib `testing`, `spec`-based or not.

For a team already committed to Ginkgo rather than migrating to `spec`,
[`gomeleon`](https://github.com/woodie/gomeleon) (formerly `ginkgo-fd`)
renders the same RSpec-style output from Ginkgo's own JSON report -- a
separate tool for the Ginkgo ecosystem specifically, not an alternative to
`spec`+`expect` for a project starting fresh.

One real gap, now closed: Ginkgo has
[`JustBeforeEach`](https://onsi.github.io/ginkgo/#separating-creation-and-configuration-justbeforeeach)
built in -- a hook that runs after every `BeforeEach` at every nesting
level, immediately before the test itself, so what varies (inputs) and the
action under test can be declared at different levels instead of
duplicated per `context`. Upstream `sclevine/spec` never had an
equivalent; `woodie/spec` adds `it.JustBeforeEach` directly (alongside
`it.BeforeEach`/`it.AfterEach`, the real names for what `it.Before`/
`it.After` already did -- those two still work, just deprecated). The
"subject" pattern below predates that addition and still works exactly
the same way -- a plain closure gets to the same place -- but
`it.JustBeforeEach` is now the more direct route when a hook, not a
closure invoked explicitly in each `it`, is what you want.

## The pieces

- **`spec`** gives you `describe`/`context`/`it` structure and
  `BeforeEach`/`AfterEach`/`JustBeforeEach` lifecycle hooks -- no
  assertions of its own.
- **`expect`** gives you Gomega-style matchers (`Equal`, `Contain`,
  `Succeed`, ...) against a plain `*testing.T`/`testing.TB` -- no BDD
  framework required, works with table-driven tests just as well.
- **`gorderly`** renders `go test -v`'s output (from either of the above,
  or neither) as a deduped, nested tree.

## Aliasing `spec`'s structural functions

`spec.Run` hands your suite closure a group parameter (`spec.G`) and `it`
(`spec.S`) positionally -- the group parameter is just a name, so call it
however reads best. The convention across these projects: name the
parameter `context`, since most nested groups describe a condition
("with a temp dir", "when the flag is present") rather than the feature
or method under test:

```go
spec.Run(t, "Object", func(t *testing.T, context spec.G, it spec.S) {
    // ...
})
```

Some suites genuinely want both words -- an outer group naming the
method under test (RSpec-style `describe "#divide"`), with `context`
nested inside for the conditions that vary. Only then, alias `describe`
to `context` once at the top of the closure -- and only then, since an
alias that's declared but never called won't compile:

```go
spec.Run(t, "Object", func(t *testing.T, context spec.G, it spec.S) {
    describe := context

    describe("DoThing", func() {
        context("with a temp dir", func() {
            // ...
        })
    })
})
```

Most files in this account only ever call `context(...)` and skip the
alias entirely -- see `gorderly`'s own `parse_test.go`/`render_test.go`.
`expect`'s `expect_test.go` and `humane`'s `time_test.go` are the
opposite case: `describe(...)` groups each matcher/feature, `context(...)`
nests conditions inside.

`it`'s hook methods (`it.BeforeEach`/`it.AfterEach`/`it.JustBeforeEach`)
are called qualified, not aliased to bare lowercase locals. Earlier
versions of this convention aliased `it.Before`/`it.After` to `before`/
`after`, but with three hook names to alias instead of two
(`beforeEach`/`afterEach`/`justBeforeEach`) the line got long and
cluttered, and abbreviating it (`it.BE`/`it.JBE`) just traded one kind of
ugly for another. `it` already reads visually distinct from the lowercase
`describe`/`context` structural vocabulary, so `it.BeforeEach(...)` etc.
called qualified doesn't clash the way a qualified `expect.Expect(...)`
would (the reason `expect` itself is dot-imported, below).

Pair this with a one-line lowercase alias for `expect`'s own `Expect`
(required because a dot-imported name has to stay capitalized, but a
local function declaration doesn't):

```go
func expect[T any](got T, t testing.TB) Expectation[T] { return Expect(got, t) }
```

Declared once per test package (see `config_test.go`), used by every `it`
in that package. See `expect`'s own README for the full reasoning behind
the argument order (`expect(got, t)`, not `expect(t, got)`).

## A literal `/` in a description string

Don't put a literal `/` in a `describe`/`context`/`it` string. `t.Run`
joins subtest names with an unescaped `/` to build the full hierarchical
name, and uses that same character to split `-run`/`-bench` patterns --
[Go's own docs are explicit](https://go.dev/blog/subtests) that a literal
`/` inside a subtest name isn't safe. This is baked into the `testing`
package itself, not something `spec`, `expect`, or `gorderly` can work
around after the fact: by the time `gorderly` sees `go test -v`'s output,
a `/` that was part of a description's own text is already
indistinguishable from a real nesting boundary. There's no delimiter
flag or escape mechanism to change that.

If a description needs to *read* like it has a slash, substitute a
lookalike character instead of the real one. `expect`'s own
`expect_test.go` does this:

```go
describe("To ̸NotTo ̸ToNot", func() {
    // ...
})
```

That's a space followed by U+0338 (`COMBINING LONG SOLIDUS OVERLAY`) --
it renders as a floating slash without ever being the ASCII `/` byte
`t.Run` treats specially, so it reads naturally while staying completely
outside `go test`'s own hierarchy syntax.

## Skipping and focusing tests

`describe`/`context` (both aliases for `spec.G`) and `it` (`spec.S`) each have
`.Pend`/`.Focus` methods -- `sclevine/spec`'s own equivalent of RSpec's/Quick's
`xdescribe`/`xit`/`fdescribe`/`fit`, just spelled as method calls instead of a
letter prefix, since Go has no bare-word `x`/`f` naming convention to lean on:

```go
context.Pend("still needs a real fixture", func() {
    // ...none of the code in this closure will run.
})

it.Pend("returns the cached value", func() {
    // ...this one spec is skipped; siblings still run.
})

context.Focus("the bug we're chasing right now", func() {
    // ...only this group (and other focused specs) run; everything else is skipped.
})
```

`it.Focus`/`context.Focus` accept the same ordering options (`spec.Random()`,
`spec.Parallel()`, ...) as a normal call; `.Pend` ignores any options passed to
it. See `xctidy`'s and `kotidy`'s own `docs/FRAMEWORK.md` for the Swift/Kotlin
equivalents (`xit`/`fit`, spelled as literal keywords there instead of methods).

## Nesting context so it's available to every sub-test

`BeforeEach` reruns fresh before each `it` -- parent context's
`BeforeEach` first, then the child's -- so a value set up at one nesting
level is visible (and freshly rebuilt) for every `it` beneath it, with no
test-to-test pollution:

```go
context("a transcript mixes pass, fail, and skip", func() {
    var pkgs []PackageResult
    var err error
    var pkg PackageResult

    it.BeforeEach(func() {
        pkgs, err = Parse(strings.NewReader(mixedTranscript))
        if len(pkgs) > 0 {
            pkg = pkgs[0]
        }
    })

    it("returns no error", func() {
        expect(err, t).To(Succeed())
    })

    it("captures the package import path and outcome", func() {
        expect(pkg.ImportPath, t).To(Equal("example.com/math"))
        expect(pkg.Outcome, t).To(Equal("FAIL"))
    })

    it("keeps only leaf results, not the container roll-up lines", func() {
        expect(len(pkg.Results), t).To(Equal(3))
    })
})
```

(`gorderly`'s own `parse_test.go`.) Every `it` reads `pkgs`/`err`/`pkg`
without redeclaring or re-running the parse itself -- that happens once,
in the shared `BeforeEach`, and each `it` only states what it's checking.

### The "subject" pattern (and `it.JustBeforeEach` as the more direct route)

Go has no `subject`/`let` keyword, but the same idea translates directly:
declare whatever `subject` depends on as plain locals in the enclosing
`describe`, define `subject` as a closure over them, and let a
`BeforeEach` at whichever level actually needs to change one set it. This
predates `it.JustBeforeEach` and still works exactly the same way:

```go
describe("FileSize", func() {
    var bytes int64
    subject := func() string { return FileSize(bytes) }

    context("with 0 bytes", func() {
        it.BeforeEach(func() { bytes = 0 })
        it("formats as Zero KB", func() {
            expect(subject(), t).To(Equal("Zero KB"))
        })
    })

    context("with a gigabyte-scale value", func() {
        it.BeforeEach(func() { bytes = 5240000000 })
        it("keeps 2 decimal places at 3 significant figures (not truncated to 1)", func() {
            expect(subject(), t).To(Equal("5.24 GB"))
        })
    })
})
```

`subject` doesn't run until called, so `subject()` inside each `it` always
reflects whatever the `BeforeEach` chain most recently set. The same shape
extends further when a `subject` closes over several independently-
overridable inputs instead of just one.

With `it.JustBeforeEach` (added in `woodie/spec` v0.2.0), the same example
reads without an explicit `subject()` call at each `it`:

```go
describe("FileSize", func() {
    var bytes int64
    var result string
    it.JustBeforeEach(func() { result = FileSize(bytes) })

    context("with 0 bytes", func() {
        it.BeforeEach(func() { bytes = 0 })
        it("formats as Zero KB", func() {
            expect(result, t).To(Equal("Zero KB"))
        })
    })

    context("with a gigabyte-scale value", func() {
        it.BeforeEach(func() { bytes = 5240000000 })
        it("keeps 2 decimal places at 3 significant figures (not truncated to 1)", func() {
            expect(result, t).To(Equal("5.24 GB"))
        })
    })
})
```

Both get to the same place. `it.JustBeforeEach` is the more direct route
when the action under test is a single call that should always run right
before the spec; the `subject` closure still reads better when a test
wants to call it multiple times or under varying conditions within a
single `it`. This matches `kwick`'s own `JustBeforeEachExtension` for
Kotest -- see `kotidy`'s own `docs/FRAMEWORK.md`.

## Mocking and stubbing

### Stubbing package state directly (no interface needed)

Not everything worth stubbing needs a full interface and fake
implementation. When a package depends on exactly one thing -- a
directory path, a single collaborator -- overriding the package-level
variable directly in a `BeforeEach` is often simpler and just as safe,
since `BeforeEach` reruns fresh for every `it`:

```go
it.BeforeEach(func() { workDir = t.TempDir() }) // stub implementation
```

The `// stub implementation` comment is a deliberate marker -- it tells a
reader this line exists to substitute test state for production state,
not because the variable would normally be assigned here.

### `httptest` for anything that talks HTTP

For handlers and middleware, stdlib's own `net/http/httptest` is usually
all the mocking needed -- no fake client type to write or maintain:

```go
describe("withLogging", func() {
    it("passes through the wrapped handler's status and body unchanged", func() {
        inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            http.Error(w, "nope", http.StatusNotFound)
        })

        req := httptest.NewRequest(http.MethodGet, "/missing", nil)
        rec := httptest.NewRecorder()
        withLogging(inner).ServeHTTP(rec, req)

        expect(rec.Code, t).To(Equal(http.StatusNotFound))
        expect(rec.Body.String(), t).To(Contain("nope"))
    })
})
```

### Test doubles for a real interface

When a dependency genuinely needs mocking (an interface with real
behavior to intercept, not just a value to swap), embed the interface and
override only the methods the test needs -- method promotion supplies the
rest for free:

```go
type spyT struct {
    testing.TB
    failed bool
}

func (s *spyT) Helper() {}
func (s *spyT) Errorf(format string, args ...interface{}) { s.failed = true }
```

(`expect`'s own `expect_test.go` -- it needs to verify a *mismatched*
assertion actually reports failure, without that failure taking down the
real test run, so it passes a `*spyT` anywhere a `testing.TB` is expected
and asserts on `spy.failed` afterward.) This shape -- embed, override,
inspect -- works for stubbing any interface this way, not just
`testing.TB`.

## A full suite, all three pieces together

```go
package myapp_test

import (
    "testing"

    "github.com/sclevine/spec"
    . "github.com/woodie/expect"
)

func expect[T any](got T, t testing.TB) Expectation[T] { return Expect(got, t) }

func TestObject(t *testing.T) {
    spec.Run(t, "Object", func(t *testing.T, context spec.G, it spec.S) {
        describe := context

        var obj *myapp.Object
        it.BeforeEach(func() { obj = myapp.NewObject(t.Context()) })
        it.AfterEach(func() { obj.Close() })

        describe("DoThing", func() {
            context("with a temp dir", func() {
                it.BeforeEach(func() { obj.Dir = t.TempDir() })

                it("succeeds", func() {
                    expect(obj.DoThing(), t).To(Succeed())
                })
            })
        })
    })
}
```

Pipe it through `go test -v ./... | gorderly -fd` and it renders as a
real, deduped, nested tree -- `gorderly` never knows or cares that
`expect` was involved, it only ever sees `go test -v`'s own output. See
`gorderly`'s own `parse_test.go`/`render_test.go` for this pattern used
against a real parser, not a sketch.

The import path stays `github.com/sclevine/spec` -- `woodie/spec` keeps
upstream's own module path unchanged, so picking up the fork's
`BeforeEach`/`AfterEach`/`JustBeforeEach` is a `go.mod` `replace`
directive, not an import change:

```
replace github.com/sclevine/spec => github.com/woodie/spec v0.2.0
```
