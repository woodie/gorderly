package main

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type TestState string

const (
	StatePass TestState = "PASS"
	StateFail TestState = "FAIL"
	StateSkip TestState = "SKIP"
)

type TestResult struct {
	Hierarchy []string
	State     TestState
	Elapsed   float64
	Output    []string
}

type PackageResult struct {
	ImportPath string
	Outcome    string
	Elapsed    float64
	Results    []TestResult
}

var (
	resultLineRe  = regexp.MustCompile(`^\s*--- (PASS|FAIL|SKIP): (\S+) \(([0-9.]+)s\)$`)
	pkgSummaryRe  = regexp.MustCompile(`^(ok|FAIL)\s+(\S+)\s+(?:([0-9.]+)s\b|\S+)`)
	noTestFilesRe = regexp.MustCompile(`^\?\s+(\S+)\s+\[no test files\]$`)
	boundaryRe    = regexp.MustCompile(`^=== (RUN|PAUSE|CONT|NAME)\s`)
)

// Parse reads raw `go test -v` output and groups it by package. Only leaf
// results are kept per package (see leavesOnly) -- Go prints a --- PASS/FAIL
// line for every t.Run at every depth, not just leaves, since a parent
// t.Run's own line only rolls up its children's outcome once they've all
// finished, unlike RSpec/Ginkgo where only `it` blocks produce a result line.
func Parse(r io.Reader) ([]PackageResult, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var packages []PackageResult
	var allNames []string
	var rawResults []TestResult
	var pending []string

	flush := func(importPath, outcome string, elapsed float64) {
		packages = append(packages, PackageResult{
			ImportPath: importPath,
			Outcome:    outcome,
			Elapsed:    elapsed,
			Results:    leavesOnly(rawResults, allNames),
		})
		allNames = nil
		rawResults = nil
		pending = nil
	}

	for scanner.Scan() {
		line := scanner.Text()

		if m := resultLineRe.FindStringSubmatch(line); m != nil {
			seconds, _ := strconv.ParseFloat(m[3], 64)
			rawResults = append(rawResults, TestResult{
				Hierarchy: strings.Split(m[2], "/"),
				State:     TestState(m[1]),
				Elapsed:   seconds,
				Output:    pending,
			})
			allNames = append(allNames, m[2])
			pending = nil
			continue
		}

		if m := pkgSummaryRe.FindStringSubmatch(line); m != nil {
			var elapsed float64
			if m[3] != "" {
				elapsed, _ = strconv.ParseFloat(m[3], 64)
			}
			flush(m[2], m[1], elapsed)
			continue
		}

		if m := noTestFilesRe.FindStringSubmatch(line); m != nil {
			packages = append(packages, PackageResult{ImportPath: m[1], Outcome: "no test files"})
			continue
		}

		if line == "PASS" || line == "FAIL" || boundaryRe.MatchString(line) {
			continue
		}

		if trimmed := strings.TrimRight(line, " \t"); trimmed != "" {
			pending = append(pending, strings.TrimLeft(trimmed, " \t"))
		}
	}

	if len(rawResults) > 0 {
		flush("", "FAIL", 0)
	}

	return packages, scanner.Err()
}

func leavesOnly(results []TestResult, allNames []string) []TestResult {
	isPrefixOfAnother := func(name string) bool {
		p := name + "/"
		for _, n := range allNames {
			if strings.HasPrefix(n, p) {
				return true
			}
		}
		return false
	}

	var leaves []TestResult
	for _, r := range results {
		if !isPrefixOfAnother(strings.Join(r.Hierarchy, "/")) {
			leaves = append(leaves, r)
		}
	}
	return leaves
}
