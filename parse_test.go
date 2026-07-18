package main

import (
	"strings"
	"testing"

	"github.com/sclevine/spec"
)

const mixedTranscript = `=== RUN   TestMath
=== RUN   TestMath/addition
=== RUN   TestMath/addition/adds_two_positive_numbers
--- PASS: TestMath/addition/adds_two_positive_numbers (0.00s)
=== RUN   TestMath/addition/adds_a_negative_number
    math_test.go:12: got 3, want 4
--- FAIL: TestMath/addition/adds_a_negative_number (0.00s)
--- FAIL: TestMath/addition (0.00s)
=== RUN   TestMath/subtraction
=== RUN   TestMath/subtraction/is_skipped_for_now
    math_test.go:20: not implemented
--- SKIP: TestMath/subtraction/is_skipped_for_now (0.00s)
--- PASS: TestMath/subtraction (0.00s)
--- FAIL: TestMath (0.00s)
FAIL
FAIL	example.com/math	0.003s
`

func TestParse(t *testing.T) {
	spec.Run(t, "Parse", func(t *testing.T, when spec.G, it spec.S) {
		when("a transcript mixes pass, fail, and skip", func() {
			var pkgs []PackageResult
			var err error
			var pkg PackageResult

			it.Before(func() {
				pkgs, err = Parse(strings.NewReader(mixedTranscript))
				if len(pkgs) > 0 {
					pkg = pkgs[0]
				}
			})

			it("returns no error", func() {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			it("groups results under one package", func() {
				if len(pkgs) != 1 {
					t.Fatalf("expected 1 package, got %d", len(pkgs))
				}
			})

			it("captures the package import path and outcome", func() {
				if pkg.ImportPath != "example.com/math" {
					t.Errorf("import path = %q", pkg.ImportPath)
				}
				if pkg.Outcome != "FAIL" {
					t.Errorf("outcome = %q", pkg.Outcome)
				}
			})

			it("keeps only leaf results, not the container roll-up lines", func() {
				if len(pkg.Results) != 3 {
					t.Fatalf("expected 3 leaf results, got %d: %+v", len(pkg.Results), pkg.Results)
				}
			})

			it("preserves each leaf's full hierarchy", func() {
				want := [][]string{
					{"TestMath", "addition", "adds_two_positive_numbers"},
					{"TestMath", "addition", "adds_a_negative_number"},
					{"TestMath", "subtraction", "is_skipped_for_now"},
				}
				for i, r := range pkg.Results {
					if strings.Join(r.Hierarchy, "/") != strings.Join(want[i], "/") {
						t.Errorf("result %d hierarchy = %v, want %v", i, r.Hierarchy, want[i])
					}
				}
			})

			it("records each leaf's state", func() {
				want := []TestState{StatePass, StateFail, StateSkip}
				for i, r := range pkg.Results {
					if r.State != want[i] {
						t.Errorf("result %d state = %s, want %s", i, r.State, want[i])
					}
				}
			})

			it("attaches captured log output to the failing leaf", func() {
				r := pkg.Results[1]
				if len(r.Output) != 1 || r.Output[0] != "math_test.go:12: got 3, want 4" {
					t.Errorf("output = %v", r.Output)
				}
			})
		})

		when("a package has no test files", func() {
			var pkgs []PackageResult
			var err error

			it.Before(func() {
				transcript := "?   \texample.com/empty\t[no test files]\n"
				pkgs, err = Parse(strings.NewReader(transcript))
			})

			it("returns no error", func() {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			it("records the package with no results", func() {
				if len(pkgs) != 1 || pkgs[0].Outcome != "no test files" {
					t.Fatalf("pkgs = %+v", pkgs)
				}
			})
		})
	})
}
