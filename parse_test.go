package main

import (
	"strings"
	"testing"
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
	t.Run("given a transcript mixing pass, fail, and skip", func(t *testing.T) {
		pkgs, err := Parse(strings.NewReader(mixedTranscript))

		t.Run("it returns no error", func(t *testing.T) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		t.Run("it groups results under one package", func(t *testing.T) {
			if len(pkgs) != 1 {
				t.Fatalf("expected 1 package, got %d", len(pkgs))
			}
		})

		pkg := pkgs[0]

		t.Run("it captures the package import path and outcome", func(t *testing.T) {
			if pkg.ImportPath != "example.com/math" {
				t.Errorf("import path = %q", pkg.ImportPath)
			}
			if pkg.Outcome != "FAIL" {
				t.Errorf("outcome = %q", pkg.Outcome)
			}
		})

		t.Run("it keeps only leaf results, not the container roll-up lines", func(t *testing.T) {
			if len(pkg.Results) != 3 {
				t.Fatalf("expected 3 leaf results, got %d: %+v", len(pkg.Results), pkg.Results)
			}
		})

		t.Run("it preserves each leaf's full hierarchy", func(t *testing.T) {
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

		t.Run("it records each leaf's state", func(t *testing.T) {
			want := []TestState{StatePass, StateFail, StateSkip}
			for i, r := range pkg.Results {
				if r.State != want[i] {
					t.Errorf("result %d state = %s, want %s", i, r.State, want[i])
				}
			}
		})

		t.Run("it attaches captured log output to the failing leaf", func(t *testing.T) {
			r := pkg.Results[1]
			if len(r.Output) != 1 || r.Output[0] != "math_test.go:12: got 3, want 4" {
				t.Errorf("output = %v", r.Output)
			}
		})
	})

	t.Run("given a package with no test files", func(t *testing.T) {
		transcript := "?   \texample.com/empty\t[no test files]\n"
		pkgs, err := Parse(strings.NewReader(transcript))

		t.Run("it returns no error", func(t *testing.T) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		t.Run("it records the package with no results", func(t *testing.T) {
			if len(pkgs) != 1 || pkgs[0].Outcome != "no test files" {
				t.Fatalf("pkgs = %+v", pkgs)
			}
		})
	})
}
