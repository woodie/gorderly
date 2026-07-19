package main

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

type Style int

const (
	StyleClassic Style = iota
	StyleFd
	StyleFs
	StyleFv
)

const (
	ansiPrefix  = "\033["
	red         = "31"
	green       = "32"
	brightGreen = "92"
	cyan        = "36"
	gray        = "90"
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
	var testFilesTotal, testFilesFailed int

	skipColor := cyan
	if style == StyleFv {
		skipColor = gray
	}

	for _, pkg := range pkgs {
		if pkg.Outcome == "no test files" {
			continue
		}
		fp("%s\n", pkg.ImportPath)
		testFilesTotal++
		pkgFailed := false

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
				pkgFailed = true
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
				fp("%s%s\n", indent, colorize(skipColor, renderLeaf(style, leafName, r, false, 0)))
			default:
				fp("%s%s\n", indent, colorizePass(style, leafName, r, colorize))
			}

			prevPath = path
		}
		if pkgFailed {
			testFilesFailed++
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

	if style == StyleFv {
		passed := total - failedCount - skipped
		filesPassed := testFilesTotal - testFilesFailed
		fp("%s\n", vitestSummaryLine("Test Files", testFilesFailed, filesPassed, 0, testFilesTotal, colorize))
		fp("%s\n", vitestSummaryLine("Tests", failedCount, passed, skipped, total, colorize))
		fp("%11s  %s\n", "Duration", formatVitestDuration(totalElapsed))
		return failedCount, writeErr
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

// vitestSummaryLine reproduces Vitest's own footer shape -- a label
// right-justified to 11 columns (matching its padSummaryTitle, which does
// str.padStart(11)), followed by "N failed | M passed | K skipped (total)",
// each count colored the same way Vitest's getStateString does. -fv exists
// so gorderly's Go output reads as a continuation of the same terminal
// Vitest already printed, not a second, differently-styled tool bolted on.
func vitestSummaryLine(label string, failed, passed, skipped, total int, colorize func(string, string) string) string {
	var parts []string
	if failed > 0 {
		parts = append(parts, colorize(red, fmt.Sprintf("%d failed", failed)))
	}
	if passed > 0 {
		parts = append(parts, colorize(green, fmt.Sprintf("%d passed", passed)))
	}
	if skipped > 0 {
		parts = append(parts, colorize(gray, fmt.Sprintf("%d skipped", skipped)))
	}
	if len(parts) == 0 {
		parts = append(parts, "0 passed")
	}
	return fmt.Sprintf("%11s  %s (%d)", label, strings.Join(parts, " | "), total)
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
	case StyleFv:
		if failed {
			return fmt.Sprintf("× %s %s", name, formatVitestDuration(r.Elapsed))
		}
		if r.State == StateSkip {
			return "↓ " + name
		}
		return "✓ " + name
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
	case StyleFv:
		num, unit := formatVitestDurationParts(r.Elapsed)
		return colorize(green, "✓") + " " + name + " " + colorize(green, num) + colorize(brightGreen, unit)
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

// formatVitestDurationParts reproduces Vitest's own formatTime (utils.ts):
// whole milliseconds under 1000ms, seconds to two decimals at or above --
// split into number and unit so callers (colorizePass, for -fv) can color
// them two different shades of green the way Vitest itself does. go test -v
// itself only reports elapsed time to two decimal places of a second (e.g.
// "0.00s"), so anything gorderly renders here under ~10ms will print as a
// flat "0ms" -- a real precision ceiling in go test's own output, not
// something -fv can recover; Vitest's per-leaf ms come from its own
// finer-grained JS timers, which go test has no equivalent of.
func formatVitestDurationParts(seconds float64) (number, unit string) {
	ms := seconds * 1000
	if ms > 1000 {
		return strconv.FormatFloat(ms/1000, 'f', 2, 64), "s"
	}
	return strconv.Itoa(int(math.Round(ms))), "ms"
}

// formatVitestDuration is formatVitestDurationParts joined back into one
// string, for the fail-leaf case (renderLeaf), which colors its whole line
// red at the call site in Render rather than shading the duration apart.
func formatVitestDuration(seconds float64) string {
	number, unit := formatVitestDurationParts(seconds)
	return number + unit
}
