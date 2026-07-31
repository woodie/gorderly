module github.com/woodie/gorderly

go 1.25.0

require (
	github.com/sclevine/spec v1.4.0
	github.com/woodie/expect v0.3.0
)

// woodie/spec is a fork of sclevine/spec adding BeforeEach/AfterEach/
// JustBeforeEach (Before/After deprecated, not removed). Module path is
// unchanged from upstream, so this is a replace, not a version bump.
replace github.com/sclevine/spec => github.com/woodie/spec v0.2.0
