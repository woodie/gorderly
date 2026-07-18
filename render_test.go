package main

import (
	"bytes"
	"strings"
	"testing"
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
	t.Run("given a package with a pass, a fail, and a skip", func(t *testing.T) {
		var buf bytes.Buffer
		failed, err := Render(samplePackages(), StyleClassic, &buf, false)
		out := buf.String()

		t.Run("it returns no write error", func(t *testing.T) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		t.Run("it reports one failed test", func(t *testing.T) {
			if failed != 1 {
				t.Errorf("failed = %d, want 1", failed)
			}
		})

		t.Run("it prints the package as a suite header", func(t *testing.T) {
			if !strings.Contains(out, "example.com/math\n") {
				t.Errorf("missing suite header in:\n%s", out)
			}
		})

		t.Run("it prints the shared context path once, not once per leaf", func(t *testing.T) {
			// Scoped to the tree, not the whole output -- the Failures section
			// below legitimately reprints each failure's full hierarchy path,
			// so counting across the entire output would double-count on purpose.
			tree, _, _ := strings.Cut(out, "Failures:")
			if strings.Count(tree, "TestMath") != 1 {
				t.Errorf("expected exactly one TestMath header line in:\n%s", tree)
			}
			if strings.Count(tree, "addition") != 1 {
				t.Errorf("expected exactly one addition header line in:\n%s", tree)
			}
		})

		t.Run("it humanizes underscored subtest names back into words", func(t *testing.T) {
			if !strings.Contains(out, "adds two positive numbers") {
				t.Errorf("missing humanized leaf name in:\n%s", out)
			}
		})

		t.Run("it ends with the shared xcbeautify-style footer", func(t *testing.T) {
			if !strings.Contains(out, "Test Failed\n") {
				t.Errorf("missing Test Failed line in:\n%s", out)
			}
			if !strings.Contains(out, "Tests Passed: 1 failed, 1 skipped, 3 total") {
				t.Errorf("missing summary line in:\n%s", out)
			}
		})

		t.Run("it lists the failure with its captured output", func(t *testing.T) {
			if !strings.Contains(out, "Failures:") {
				t.Errorf("missing Failures section in:\n%s", out)
			}
			if !strings.Contains(out, "math_test.go:12: got 3, want 4") {
				t.Errorf("missing captured failure output in:\n%s", out)
			}
		})
	})

	t.Run("given every test passes", func(t *testing.T) {
		pkgs := []PackageResult{{
			ImportPath: "example.com/clean",
			Outcome:    "ok",
			Results: []TestResult{
				{Hierarchy: []string{"TestClean", "does the thing"}, State: StatePass, Elapsed: 0.001},
			},
		}}
		var buf bytes.Buffer
		failed, _ := Render(pkgs, StyleClassic, &buf, false)

		t.Run("it reports zero failures", func(t *testing.T) {
			if failed != 0 {
				t.Errorf("failed = %d, want 0", failed)
			}
		})

		t.Run("it closes with Test Succeeded, not Test Failed", func(t *testing.T) {
			if !strings.Contains(buf.String(), "Test Succeeded\n") {
				t.Errorf("missing Test Succeeded line in:\n%s", buf.String())
			}
		})

		t.Run("it omits the Failures section entirely", func(t *testing.T) {
			if strings.Contains(buf.String(), "Failures:") {
				t.Errorf("unexpected Failures section in:\n%s", buf.String())
			}
		})
	})

	t.Run("when color is disabled", func(t *testing.T) {
		var buf bytes.Buffer
		_, _ = Render(samplePackages(), StyleClassic, &buf, false)

		t.Run("it omits ANSI escape codes", func(t *testing.T) {
			if strings.Contains(buf.String(), "\033[") {
				t.Errorf("unexpected ANSI codes in:\n%s", buf.String())
			}
		})
	})

	t.Run("in fd style", func(t *testing.T) {
		var buf bytes.Buffer
		_, _ = Render(samplePackages(), StyleFd, &buf, false)
		out := buf.String()

		t.Run("it omits the classic glyph", func(t *testing.T) {
			if strings.Contains(out, "✔") || strings.Contains(out, "✖") {
				t.Errorf("unexpected glyph in fd style:\n%s", out)
			}
		})

		t.Run("it labels the skipped leaf PENDING", func(t *testing.T) {
			if !strings.Contains(out, "(PENDING)") {
				t.Errorf("missing (PENDING) label in:\n%s", out)
			}
		})
	})

	t.Run("in fs style", func(t *testing.T) {
		var buf bytes.Buffer
		_, _ = Render(samplePackages(), StyleFs, &buf, false)
		out := buf.String()

		t.Run("it uses a checkmark for the passing leaf", func(t *testing.T) {
			if !strings.Contains(out, "✔") {
				t.Errorf("missing checkmark in fs style:\n%s", out)
			}
		})
	})
}
