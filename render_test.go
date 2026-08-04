package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sclevine/spec"
	. "github.com/woodie/expect"
)

func samplePackages() []PackageResult {
	return []PackageResult{
		{
			ImportPath: "example.com/math",
			Outcome:    "FAIL",
			Results: []TestResult{
				{Hierarchy: []string{"TestMath", "addition", "adds_two_positive_numbers"}, State: StatePass, Elapsed: 0.001},
				{Hierarchy: []string{"TestMath", "addition", "adds_a_negative_number"}, State: StateFail, Elapsed: 0.002, Output: []string{"math_test.go:12: got 3, want 4"}},
				{Hierarchy: []string{"TestMath", "subtraction", "is_skipped_for_now"}, State: StateSkip, Elapsed: 0},
			},
		},
	}
}

func TestRender(t *testing.T) {
	spec.Run(t, "Render", func(t *testing.T, context spec.G, it spec.S) {
		context("a package has a pass, a fail, and a skip", func() {
			var buf bytes.Buffer
			var failed int
			var err error
			var out string

			it.Before(func() {
				failed, err = Render(samplePackages(), StyleClassic, &buf, false)
				out = buf.String()
			})

			it("returns no write error", func() {
				expect(err, t).To(Succeed())
			})

			it("reports one failed test", func() {
				expect(failed, t).To(Equal(1))
			})

			it("prints the package as a suite header", func() {
				expect(out, t).To(Contain("example.com/math\n"))
			})

			it("prints the top-level test function flush left, not nested under the package header", func() {
				// The package path is informational, not a real node in go
				// test's own hierarchy -- it should cost no indent level.
				expect(out, t).To(Contain("example.com/math\n\nTestMath\n  addition\n"))
			})

			it("prints the shared context path once, not once per leaf", func() {
				// Scoped to the tree, not the whole output -- the Failures section
				// below legitimately reprints each failure's full hierarchy path,
				// so counting across the entire output would double-count on purpose.
				tree, _, _ := strings.Cut(out, "Failures:")
				expect(strings.Count(tree, "TestMath"), t).To(Equal(1))
				expect(strings.Count(tree, "addition"), t).To(Equal(1))
			})

			it("humanizes underscored subtest names back into words", func() {
				expect(out, t).To(Contain("adds two positive numbers"))
			})

			it("ends with the shared xcbeautify-style footer", func() {
				expect(out, t).To(Contain("Test Failed\n"))
				expect(out, t).To(Contain("Tests Passed: 1 failed, 1 skipped, 3 total"))
			})

			it("lists the failure with its captured output", func() {
				expect(out, t).To(Contain("Failures:"))
				expect(out, t).To(Contain("math_test.go:12: got 3, want 4"))
			})

			it("marks the failing leaf with a FAILED cross-reference to the Failures section", func() {
				expect(out, t).To(Contain("adds a negative number (FAILED - 1) (0.0020 seconds)"))
			})

			it("marks the skipped leaf with its elapsed time and no SKIPPED text", func() {
				expect(out, t).To(Contain("⊘ is skipped for now (0.0000 seconds)"))
				expect(out, t).NotTo(Contain("SKIPPED"))
			})
		})

		context("a package has more than one top-level Test function", func() {
			var out string

			it.Before(func() {
				pkgs := []PackageResult{{
					ImportPath: "example.com/multi",
					Outcome:    "ok",
					Results: []TestResult{
						{Hierarchy: []string{"TestMath", "addition", "adds_two_positive_numbers"}, State: StatePass, Elapsed: 0.001},
						{Hierarchy: []string{"TestGeometry", "area", "computes_a_rectangle"}, State: StatePass, Elapsed: 0.001},
					},
				}}
				var buf bytes.Buffer
				_, _ = Render(pkgs, StyleClassic, &buf, false)
				out = buf.String()
			})

			it("separates the two top-level suites with a blank line", func() {
				expect(out, t).To(Contain("adds two positive numbers (0.0010 seconds)\n\nTestGeometry"))
			})

			it("also separates the package header from the first suite with a blank line", func() {
				expect(out, t).To(Contain("example.com/multi\n\nTestMath"))
			})
		})

		context("classic style with color enabled", func() {
			var out string

			it.Before(func() {
				var buf bytes.Buffer
				_, _ = Render(samplePackages(), StyleClassic, &buf, true)
				out = buf.String()
			})

			it("colors only the glyph and the elapsed time on a passing leaf, not the name", func() {
				expect(out, t).To(Contain("\033[32m✔\033[0m adds two positive numbers (\033[32m0.0010\033[0m seconds)"))
			})

			it("colors only the glyph and the elapsed time on a failing leaf, not the name or FAILED marker", func() {
				expect(out, t).To(Contain("\033[31m✖\033[0m adds a negative number (FAILED - 1) (\033[31m0.0020\033[0m seconds)"))
			})

			it("colors only the glyph and the elapsed time on a skipped leaf, not the name", func() {
				expect(out, t).To(Contain("\033[36m⊘\033[0m is skipped for now (\033[36m0.0000\033[0m seconds)"))
			})
		})

		context("every test passes", func() {
			var buf bytes.Buffer
			var failed int

			it.Before(func() {
				pkgs := []PackageResult{{
					ImportPath: "example.com/clean",
					Outcome:    "ok",
					Results: []TestResult{
						{Hierarchy: []string{"TestClean", "does the thing"}, State: StatePass, Elapsed: 0.001},
					},
				}}
				failed, _ = Render(pkgs, StyleClassic, &buf, false)
			})

			it("reports zero failures", func() {
				expect(failed, t).To(Equal(0))
			})

			it("closes with Test Succeeded, not Test Failed", func() {
				expect(buf.String(), t).To(Contain("Test Succeeded\n"))
			})

			it("omits the Failures section entirely", func() {
				expect(buf.String(), t).NotTo(Contain("Failures:"))
			})
		})

		context("color is disabled", func() {
			var buf bytes.Buffer

			it.Before(func() {
				_, _ = Render(samplePackages(), StyleClassic, &buf, false)
			})

			it("omits ANSI escape codes", func() {
				expect(buf.String(), t).NotTo(Contain("\033["))
			})
		})

		context("in fd style", func() {
			var buf bytes.Buffer
			var out string

			it.Before(func() {
				_, _ = Render(samplePackages(), StyleFd, &buf, false)
				out = buf.String()
			})

			it("omits the classic glyph", func() {
				expect(out, t).NotTo(Contain("✔"))
				expect(out, t).NotTo(Contain("✖"))
			})

			it("labels the skipped leaf PENDING", func() {
				expect(out, t).To(Contain("(PENDING)"))
			})

			it("colors pending yellow, not cyan, when a TTY", func() {
				var buf bytes.Buffer
				_, _ = Render(samplePackages(), StyleFd, &buf, true)
				expect(buf.String(), t).To(Contain("\033[33m"))
				expect(buf.String(), t).NotTo(Contain("\033[36m"))
			})
		})

		context("in fs style", func() {
			var buf bytes.Buffer
			var out string

			it.Before(func() {
				_, _ = Render(samplePackages(), StyleFs, &buf, false)
				out = buf.String()
			})

			it("uses a checkmark for the passing leaf", func() {
				expect(out, t).To(Contain("✔"))
			})

			it("uses a cross and keeps the FAILED cross-reference for the failing leaf", func() {
				expect(out, t).To(Contain("✗ adds a negative number (FAILED - 1)"))
			})

			it("uses a dash and keeps the SKIPPED marker for the skipped leaf", func() {
				expect(out, t).To(Contain("- is skipped for now (SKIPPED)"))
			})
		})

		context("in fv style", func() {
			var buf bytes.Buffer
			var out string

			it.Before(func() {
				_, _ = Render(samplePackages(), StyleFv, &buf, false)
				out = buf.String()
			})

			it("uses Vitest's own glyphs for pass, fail, and skip", func() {
				expect(out, t).To(Contain("✓ adds two positive numbers"))
				expect(out, t).To(Contain("× adds a negative number"))
				expect(out, t).To(Contain("↓ is skipped for now"))
			})

			it("closes with a Vitest-shaped Test Files, Tests, and Duration footer", func() {
				expect(out, t).To(Contain("Test Files  1 failed (1)"))
				expect(out, t).To(Contain("Tests  1 failed | 1 passed | 1 skipped (3)"))
				expect(out, t).To(Contain("Duration  "))
			})

			it("omits the RSpec-style Test Succeeded or Test Failed verdict line", func() {
				expect(out, t).NotTo(Contain("Test Succeeded"))
				expect(out, t).NotTo(Contain("Test Failed"))
			})

			it("shows per-leaf elapsed time in milliseconds", func() {
				expect(out, t).To(Contain("✓ adds two positive numbers 1ms"))
				expect(out, t).To(Contain("× adds a negative number 2ms"))
				expect(out, t).NotTo(Contain("↓ is skipped for now "))
			})
		})

		context("in fv style with a leaf slower than one second", func() {
			var buf bytes.Buffer
			var out string

			it.Before(func() {
				pkgs := []PackageResult{{
					ImportPath: "example.com/slow",
					Outcome:    "ok",
					Results: []TestResult{
						{Hierarchy: []string{"TestSlow", "takes a while"}, State: StatePass, Elapsed: 1.5},
					},
				}}
				_, _ = Render(pkgs, StyleFv, &buf, false)
				out = buf.String()
			})

			it("switches from milliseconds to seconds", func() {
				expect(out, t).To(Contain("✓ takes a while 1.50s"))
			})
		})

		context("in fv style with every test passing", func() {
			var buf bytes.Buffer
			var out string

			it.Before(func() {
				pkgs := []PackageResult{{
					ImportPath: "example.com/clean",
					Outcome:    "ok",
					Results: []TestResult{
						{Hierarchy: []string{"TestClean", "does the thing"}, State: StatePass, Elapsed: 0.001},
					},
				}}
				_, _ = Render(pkgs, StyleFv, &buf, false)
				out = buf.String()
			})

			it("reports one passing test file and one passing test", func() {
				expect(out, t).To(Contain("Test Files  1 passed (1)"))
				expect(out, t).To(Contain("Tests  1 passed (1)"))
			})
		})

		context("every leaf rounds down to zero but the package itself took real time", func() {
			// go test -v's own per-leaf lines are rounded to two decimal
			// places, so a package of sub-millisecond tests reports "0.00s"
			// for every one of them -- summing those leaves (the old bug)
			// produced a total of exactly zero. pkg.Elapsed simulates go
			// test's own "ok <pkg> 0.363s" package-summary line, a real
			// measurement the rounded leaves never had.
			samplePkgs := func() []PackageResult {
				return []PackageResult{{
					ImportPath: "example.com/fast",
					Outcome:    "ok",
					Elapsed:    0.363,
					Results: []TestResult{
						{Hierarchy: []string{"TestFast", "does the first thing"}, State: StatePass, Elapsed: 0},
						{Hierarchy: []string{"TestFast", "does the second thing"}, State: StatePass, Elapsed: 0},
					},
				}}
			}

			it("reflects real elapsed time in the fv Duration footer", func() {
				var buf bytes.Buffer
				_, _ = Render(samplePkgs(), StyleFv, &buf, false)
				expect(buf.String(), t).To(Contain("Duration  363ms"))
			})

			it("reflects real elapsed time in the classic Tests Passed footer", func() {
				var buf bytes.Buffer
				_, _ = Render(samplePkgs(), StyleClassic, &buf, false)
				expect(buf.String(), t).To(Contain("(0.3630 seconds)"))
			})
		})
	})
}
