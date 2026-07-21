package main

import (
	"testing"

	"github.com/sclevine/spec"
	. "github.com/woodie/expect"
)

func TestWantsVersion(t *testing.T) {
	spec.Run(t, "wantsVersion", func(t *testing.T, describe spec.G, it spec.S) {
		context := describe

		context("the long flag is present", func() {
			it("matches", func() {
				expect(wantsVersion([]string{"--version"}), t).To(Equal(true))
			})
		})

		context("the short flag is present", func() {
			it("matches", func() {
				expect(wantsVersion([]string{"-v"}), t).To(Equal(true))
			})
		})

		context("the flag is mixed in among other args", func() {
			it("matches regardless of position", func() {
				expect(wantsVersion([]string{"-fd", "--version", "./..."}), t).To(Equal(true))
				expect(wantsVersion([]string{"./...", "-v"}), t).To(Equal(true))
			})
		})

		context("neither flag is present", func() {
			it("does not match other flags or positionals", func() {
				expect(wantsVersion([]string{"-fd", "./..."}), t).To(Equal(false))
				expect(wantsVersion([]string{"--format", "spec"}), t).To(Equal(false))
			})

			it("does not match an empty argument list", func() {
				expect(wantsVersion([]string{}), t).To(Equal(false))
			})
		})
	})
}
