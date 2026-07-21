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
	yellow      = "33"
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
				fp("%s%s\n", indent, colorizeFail(style, leafName, r, n, colorize))
			case StateSkip:
				skipped++
				fp("%s%s\n", indent, colorizeSkip(style, leafName, r, colorize))
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

// colorizePass, colorizeFail, and colorizeSkip each cover one test state
// across all four styles -- matching xctidy's Engine.swift labelForPassed/
// labelForSkipped/labelForFailed function-per-state split exactly, rather
// than one shared renderLeaf shape with a generic outer color wrap. That
// split matters because the styles don't just differ in glyph/text: .classic
// colors only the glyph and the elapsed-time number, leaving the name and
// surrounding text in the terminal's default color, while .fd/.fs color the
// whole label as one block -- a single wrap-the-whole-string helper can't
// express both shapes at once.

func colorizePass(style Style, name string, r TestResult, colorize func(string, string) string) string {
	switch style {
	case StyleClassic:
		return colorize(green, "✔") + " " + name + " (" + colorize(green, formatSeconds(r.Elapsed)) + " seconds)"
	case StyleFs:
		return colorize(green, "✔") + " " + colorize(gray, name)
	case StyleFv:
		num, unit := formatVitestDurationParts(r.Elapsed)
		return colorize(green, "✓") + " " + name + " " + colorize(green, num) + colorize(brightGreen, unit)
	default: // StyleFd
		return colorize(green, name)
	}
}

func colorizeFail(style Style, name string, r TestResult, n int, colorize func(string, string) string) string {
	switch style {
	case StyleClassic:
		return colorize(red, "✖") + " " + name + fmt.Sprintf(" (FAILED - %d)", n) +
			" (" + colorize(red, formatSeconds(r.Elapsed)) + " seconds)"
	case StyleFs:
		return colorize(red, fmt.Sprintf("✗ %s (FAILED - %d)", name, n))
	case StyleFv:
		// No inline "(FAILED - N)" -- Vitest's own tree doesn't number failures
		// inline either; the trailing Failures: section still cross-references
		// by number, same as xctidy's -fv.
		return colorize(red, fmt.Sprintf("× %s %s", name, formatVitestDuration(r.Elapsed)))
	default: // StyleFd
		return colorize(red, fmt.Sprintf("%s (FAILED - %d)", name, n))
	}
}

func colorizeSkip(style Style, name string, r TestResult, colorize func(string, string) string) string {
	switch style {
	case StyleClassic:
		return colorize(cyan, "⊘") + " " + name + " (" + colorize(cyan, formatSeconds(r.Elapsed)) + " seconds)"
	case StyleFs:
		return colorize(cyan, fmt.Sprintf("- %s (SKIPPED)", name))
	case StyleFv:
		return colorize(gray, "↓ "+name)
	default: // StyleFd
		return colorize(yellow, fmt.Sprintf("%s (PENDING)", name))
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
// string, for the fail-leaf case (colorizeFail's StyleFv branch), which
// colors its whole label red rather than shading the duration apart.
func formatVitestDuration(seconds float64) string {
	number, unit := formatVitestDurationParts(seconds)
	return number + unit
}
