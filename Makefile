.PHONY: build install lint test check

build:
	go build -o gorderly

install: build
	mv gorderly ~/go/bin/

# lint and test are always verbose. check is terse: suppress everything on
# success, dump the full log on any failure -- matching the intent of every
# other lint/test/check split in this account (see xctidy's, humane's,
# next-caltrain-{kotlin,swift}'s Makefiles).

lint:
	golangci-lint run

# Verbose on purpose, and dogfoods gorderly on its own suite -- the same
# self-hosting xctidy does against its own Quick/Nimble specs.
test:
	go run . ./...

# Terser than `test` on purpose: plain `go test` has no per-test dot mode of
# its own (unlike ginkgo's free dots, which `humane`'s check target relies
# on) -- this just suppresses output on success and dumps the full log on
# failure, guaranteeing errors are never hidden regardless of go test's
# exact output.
check: lint
	@LOG=$$(mktemp); \
	if go test ./... > "$$LOG" 2>&1; then \
		echo "PASS"; \
	else \
		cat "$$LOG"; \
		rm -f "$$LOG"; \
		exit 1; \
	fi; \
	rm -f "$$LOG"
