# Writing tests: `spec` + `expect`

How we structure Go tests across these projects (`gorderly` itself,
[`lambada`](https://github.com/woodie/lambada),
[`humane`](https://github.com/woodie/humane)) -- structural functions and
lifecycle hooks from [`sclevine/spec`](https://github.com/sclevine/spec),
matchers from [`expect`](https://github.com/woodie/expect), and
`gorderly` rendering whatever `go test -v` prints as a real tree. Each
piece is independent -- `spec` needs no assertion library, `expect` needs
no BDD framework, `gorderly` only ever needs `go test -v`'s raw text --
but they're built to be used together, and this doc shows what that looks
like in real suites. The Swift side of this pairing (`xctidy`, `zouk`,
`next-caltrain-swift`) follows the same shape with different tools -- see
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

## The pieces

- **`spec`** gives you `describe`/`context`/`it` structure and `before`/
  `after` lifecycle hooks -- no assertions of its own.
- **`expect`** gives you Gomega-style matchers (`Equal`, `Contain`,
  `Succeed`, ...) against a plain `*testing.T`/`testing.TB` -- no BDD
  framework required, works with table-driven tests just as well.
- **`gorderly`** renders `go test -v`'s output (from either of the above,
  or neither) as a deduped, nested tree.

## Aliasing `spec`'s structural functions

`spec.Run` hands your suite closure `describe`/`context`/`it` positionally
-- they're just parameter names, so name them however reads best. The
convention across these projects: alias `it.Before`/`it.After` to
`before`/`after` once at the top of the closure, so the whole suite reads
in lowercase RSpec-style vocabulary instead of `it.Before(...)`/
`it.After(...)` at every call site:

```go
spec.Run(t, "Object", func(t *testing.T, describe spec.G, it spec.S) {
    context, before, after := describe, it.Before, it.After
    // ...
})
```

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

## Nesting context so it's available to every sub-test

`before` reruns fresh before each `it` -- parent context's `before` first,
then the child's -- so a value set up at one nesting level is visible
(and freshly rebuilt) for every `it` beneath it, with no test-to-test
pollution:

```go
context("a transcript mixes pass, fail, and skip", func() {
    var pkgs []PackageResult
    var err error
    var pkg PackageResult

    before(func() {
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
in the shared `before`, and each `it` only states what it's checking.

### The "subject" pattern

Go has no `subject`/`let` keyword, but the same idea translates directly:
declare whatever `subject` depends on as plain locals in the enclosing
`describe`, define `subject` as a closure over them, and let a `before` at
whichever level actually needs to change one set it.

```go
describe("HumanSize", func() {
    var bytes int64
    subject := func() string { return humane.HumanSize(bytes) }

    context("with 0 bytes", func() {
        before(func() { bytes = 0 })
        it("formats as Zero KB, matching ByteCountFormatter's own wording", func() {
            expect(subject(), t).To(Equal("Zero KB"))
        })
    })

    context("with a gigabyte-scale value", func() {
        before(func() { bytes = 5240000000 })
        it("keeps 2 decimal places at 3 significant figures (not truncated to 1)", func() {
            expect(subject(), t).To(Equal("5.24 GB"))
        })
    })
})
```

(`humane`'s own `size_test.go` -- see also `time_test.go` for a `subject`
closing over several independently-overridable inputs, not just one.)
`subject` doesn't run until called, so `subject()` inside each `it` always
reflects whatever the `before` chain most recently set.

## Mocking and stubbing

### Stubbing package state directly (no interface needed)

Not everything worth stubbing needs a full interface and fake
implementation. When a package depends on exactly one thing -- a
directory path, a single collaborator -- overriding the package-level
variable directly in a `before` is often simpler and just as safe, since
`before` reruns fresh for every `it`:

```go
before(func() { attachmentDir = t.TempDir() }) // stub implementation
```

(`lambada`'s `attachments_test.go`/`scanfiles_test.go` both do this for
their respective directory variables.) The `// stub implementation`
comment is a deliberate marker -- it tells a reader this line exists to
substitute test state for production state, not because the variable
would normally be assigned here.

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

(`lambada`'s `middleware_test.go`.)

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

Pipe it through `go test -v ./... | gorderly -fd` and it renders as a
real, deduped, nested tree -- `gorderly` never knows or cares that
`expect` was involved, it only ever sees `go test -v`'s own output. See
`gorderly`'s own `parse_test.go`/`render_test.go` for this pattern used
against a real parser, not a sketch.
