package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sclevine/spec"
	. "github.com/woodie/expect"
)

func TestRun(t *testing.T) {
	spec.Run(t, "run", func(t *testing.T, describe spec.G, it spec.S) {
		context := describe

		context("--version is passed", func() {
			it("prints the version and exits 0 without reading stdin", func() {
				var stdout, stderr bytes.Buffer
				code := run([]string{"--version"}, strings.NewReader(""), &stdout, &stderr)
				expect(code, t).To(Equal(0))
				expect(stdout.String(), t).To(Equal(gorderlyVersion + "\n"))
			})
		})

		context("-v is passed among other args", func() {
			it("still prints the version and exits 0", func() {
				var stdout, stderr bytes.Buffer
				code := run([]string{"-fd", "-v"}, strings.NewReader(""), &stdout, &stderr)
				expect(code, t).To(Equal(0))
				expect(stdout.String(), t).To(Equal(gorderlyVersion + "\n"))
			})
		})
	})
}
