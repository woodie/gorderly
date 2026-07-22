package main

// gorderlyVersion is bumped by hand at each tagged release, unlike xctidy's
// git-describe-derived Version.swift -- gorderly's primary install path is
// `go install github.com/woodie/gorderly@latest`, a module-proxy fetch with
// no .git metadata to describe, so the version has to already be the right
// string in the committed source at tag time rather than regenerated from a
// local clone at build time.
const gorderlyVersion = "0.3.2"

// wantsVersion mirrors xctidy's own wantsVersion: checked before reading
// stdin, so a bare `gorderly --version` doesn't hang waiting for piped input
// that will never arrive.
func wantsVersion(args []string) bool {
	for _, a := range args {
		if a == "--version" || a == "-v" {
			return true
		}
	}
	return false
}
