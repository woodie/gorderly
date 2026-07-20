package main

import (
	"strings"
	"testing"

	"github.com/sclevine/spec"
	. "github.com/woodie/expect"
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
	spec.Run(t, "Parse", func(t *testing.T, describe spec.G, it spec.S) {

		context, before, _ := describe, it.Before, it.After

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

			it("groups results under one package", func() {
				expect(len(pkgs), t).To(Equal(1))
			})

			it("captures the package import path and outcome", func() {
				expect(pkg.ImportPath, t).To(Equal("example.com/math"))
				expect(pkg.Outcome, t).To(Equal("FAIL"))
			})

			it("keeps only leaf results, not the container roll-up lines", func() {
				expect(len(pkg.Results), t).To(Equal(3))
			})

			it("preserves each leaf's full hierarchy", func() {
				want := [][]string{
					{"TestMath", "addition", "adds_two_positive_numbers"},
					{"TestMath", "addition", "adds_a_negative_number"},
					{"TestMath", "subtraction", "is_skipped_for_now"},
				}
				for i, r := range pkg.Results {
					expect(r.Hierarchy, t).To(DeepEqual(want[i]))
				}
			})

			it("records each leaf's state", func() {
				want := []TestState{StatePass, StateFail, StateSkip}
				for i, r := range pkg.Results {
					expect(r.State, t).To(Equal(want[i]))
				}
			})

			it("attaches captured log output to the failing leaf", func() {
				r := pkg.Results[1]
				expect(r.Output, t).To(DeepEqual([]string{"math_test.go:12: got 3, want 4"}))
			})
		})

		context("a package has no test files", func() {
			var pkgs []PackageResult
			var err error

			before(func() {
				transcript := "?   \texample.com/empty\t[no test files]\n"
				pkgs, err = Parse(strings.NewReader(transcript))
			})

			it("returns no error", func() {
				expect(err, t).To(Succeed())
			})

			it("records the package with no results", func() {
				expect(len(pkgs), t).To(Equal(1))
				if len(pkgs) == 1 {
					expect(pkgs[0].Outcome, t).To(Equal("no test files"))
				}
			})
		})
	})
}
