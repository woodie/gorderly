package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Style int

const (
	StyleClassic Style = iota
	StyleFd
	StyleFs
)

const (
	ansiPrefix = "\033["
	red        = "31"
	green      = "32"
	cyan       = "36"
	gray       = "90"
)

type failureEntry struct {
	n       int
	full    []string
	output  []string
}

// Render walks each package's leaves, deduping the shared context path
// against the previous leaf the same way xctidy's Engine and next-caltrain-
// kotlin's Gradle TestListener do, then closes with one shared footer --
// xctidy dropped stacking a second native-formatter summary on top of it
// after real side-by-side output made mixed footers "confusing"; gorderly
// follows that same resolved lesson from the start rather than relearning it.
func Render(pkgs []PackageResult, style Style, out io.Writer, colorEnabled bool) (failedCount int, err error) {
	colorize := func(code, s string) string {
		if !colorEnabled {
			return s
		}
		return ansiPrefix + code + "m" + s + ansiPrefix + "0m"
	}

	var writeErr error
	fp := func(format string, a ...any) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintf(out, format, a...)
	}

	var total, skipped int
	var totalElapsed float64
	var failures []failureEntry

	for _, pkg := range pkgs {
		if pkg.Outcome == "no test files" {
			continue
		}
		fp("%s\n", pkg.ImportPath)

		var prevPath []string
		for _, r := range pkg.Results {
			path := r.Hierarchy[:len(r.Hierarchy)-1]
			leafName := humanize(r.Hierarchy[len(r.Hierarchy)-1])

			shared := 0
			for shared < len(prevPath) && shared < len(path) && prevPath[shared] == path[shared] {
				shared++
			}
			for i := shared; i < len(path); i++ {
				fp("%s%s\n", strings.Repeat("  ", i+1), humanize(path[i]))
			}

			total++
			totalElapsed += r.Elapsed

			indent := strings.Repeat("  ", len(path)+1)
			switch r.State {
			case StateFail:
				failedCount++
				n := len(failures) + 1
				humanized := make([]string, len(r.Hierarchy))
				for i, h := range r.Hierarchy {
					humanized[i] = humanize(h)
				}
				failures = append(failures, failureEntry{
					n:      n,
					full:   append([]string{pkg.ImportPath}, humanized...),
					output: r.Output,
				})
				fp("%s%s\n", indent, colorize(red, renderLeaf(style, leafName, r, true, n)))
			case StateSkip:
				skipped++
				fp("%s%s\n", indent, colorize(cyan, renderLeaf(style, leafName, r, false, 0)))
			default:
				fp("%s%s\n", indent, colorizePass(style, leafName, r, colorize))
			}

			prevPath = path
		}
		fp("\n")
	}

	if len(failures) > 0 {
		fp("Failures:\n")
		for _, f := range failures {
			fp("\n  %d) %s\n", f.n, strings.Join(f.full, " "))
			for _, line := range f.output {
				fp("     %s\n", line)
			}
		}
		fp("\n")
	}

	verdict, verdictColor := "Test Succeeded", green
	if failedCount > 0 {
		verdict, verdictColor = "Test Failed", red
	}
	fp("%s\n", colorize(verdictColor, verdict))
	fp("%s\n", colorize(verdictColor, fmt.Sprintf(
		"Tests Passed: %d failed, %d skipped, %d total (%s seconds)",
		failedCount, skipped, total, formatSeconds(totalElapsed),
	)))

	return failedCount, writeErr
}

// renderLeaf covers the FAIL/SKIP text; passing leaves need their own color
// handling since .classic keeps the glyph but .fd/.fs don't, so that one
// case can't share this helper's plain-text shape.
func renderLeaf(style Style, name string, r TestResult, failed bool, n int) string {
	switch style {
	case StyleFd:
		if failed {
			return fmt.Sprintf("%s (FAILED - %d)", name, n)
		}
		if r.State == StateSkip {
			return fmt.Sprintf("%s (PENDING)", name)
		}
		return name
	case StyleFs:
		if failed {
			return "✖ " + name
		}
		if r.State == StateSkip {
			return "○ " + name
		}
		return name
	default:
		glyph := "✔"
		if failed {
			glyph = "✖"
		} else if r.State == StateSkip {
			glyph = "⊘"
		}
		return fmt.Sprintf("%s %s (%s seconds)", glyph, name, formatSeconds(r.Elapsed))
	}
}

func colorizePass(style Style, name string, r TestResult, colorize func(string, string) string) string {
	switch style {
	case StyleFs:
		return colorize(green, "✔") + " " + colorize(gray, name)
	default:
		return colorize(green, renderLeaf(style, name, r, false, 0))
	}
}

// humanize reverses go test's own space-to-underscore substitution in
// t.Run names -- known-imprecise for names with genuine underscores, same
// tradeoff xctidy/ginkgo-fd accept for their own comma/prose heuristics.
func humanize(s string) string {
	return strings.ReplaceAll(s, "_", " ")
}

// Matches ginkgo-fd's/xctidy's own precision split: sub-second runs (the
// overwhelming majority of unit tests) get enough decimals to be non-zero,
// anything slower rounds to hundredths instead of a false-precision tail.
func formatSeconds(seconds float64) string {
	if seconds < 1 {
		return strconv.FormatFloat(seconds, 'f', 4, 64)
	}
	return strconv.FormatFloat(seconds, 'f', 2, 64)
}
