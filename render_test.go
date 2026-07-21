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
	spec.Run(t, "Render", func(t *testing.T, describe spec.G, it spec.S) {
		context, before, _ := describe, it.Before, it.After

		context("a package has a pass, a fail, and a skip", func() {
			var buf bytes.Buffer
			var failed int
			var err error
			var out string

			before(func() {
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

		context("classic style with color enabled", func() {
			var out string

			before(func() {
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

			before(func() {
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

			before(func() {
				_, _ = Render(samplePackages(), StyleClassic, &buf, false)
			})

			it("omits ANSI escape codes", func() {
				expect(buf.String(), t).NotTo(Contain("\033["))
			})
		})

		context("in fd style", func() {
			var buf bytes.Buffer
			var out string

			before(func() {
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

			before(func() {
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

			before(func() {
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

			it("shows elapsed time in milliseconds on pass and fail leaves, like Vitest's own tree", func() {
				expect(out, t).To(Contain("✓ adds two positive numbers 1ms"))
				expect(out, t).To(Contain("× adds a negative number 2ms"))
				expect(out, t).NotTo(Contain("↓ is skipped for now "))
			})
		})

		context("in fv style with a leaf slower than one second", func() {
			var buf bytes.Buffer
			var out string

			before(func() {
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

			it("switches from milliseconds to seconds, matching Vitest's own formatTime threshold", func() {
				expect(out, t).To(Contain("✓ takes a while 1.50s"))
			})
		})

		context("in fv style with every test passing", func() {
			var buf bytes.Buffer
			var out string

			before(func() {
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
	})
}
