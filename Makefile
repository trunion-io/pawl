# pawl — build, test and release.
#
# Everything here assumes CGO_ENABLED=0. pawl ships as a static binary with no
# runtime dependency, and cgo would link the host libc and silently break that.

BINARY    := pawl
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

export CGO_ENABLED := 0

# -trimpath strips local filesystem paths, which is what makes rebuilds
# byte-identical. For a tool whose product is provenance, a client being able to
# rebuild from source and match our checksum is worth more than it costs.
# -buildvcs=false because Go otherwise stamps vcs.revision, vcs.time and
# vcs.modified into every binary, which makes the output depend on the state of
# the working tree rather than on the source. PAWL-013 AC11 promises a third
# party can rebuild a tag byte-for-byte; under VCS stamping that holds only if
# their clone is pristine and fails outright for anyone building from a source
# tarball, which has no .git at all. Provenance belongs in the signature and the
# attestation, not smuggled into the artifact in a way that breaks the property
# the attestation is asserting.
GOFLAGS := -trimpath -buildvcs=false
LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := build

.PHONY: build
build: ## Build ./bin/pawl for this machine
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/pawl

.PHONY: install
install: ## Install into GOBIN (./bin under direnv)
	go install $(GOFLAGS) -ldflags '$(LDFLAGS)' ./cmd/pawl

.PHONY: test
test: ## Run the end-to-end suite against real git repos
	go test ./... -count=1

.PHONY: fmt
fmt: ## Report unformatted files
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then echo "unformatted:"; echo "$$out"; exit 1; fi

.PHONY: vet
vet:
	go vet ./...

.PHONY: check
check: fmt vet test check-pins check-tagger ## What CI runs

.PHONY: dist
FUZZTIME ?= 30s

dist: ## Cross-compile every supported platform + checksums
	@rm -rf dist && mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=""; \
		[ "$$os" = windows ] && ext=".exe"; \
		out="dist/$(BINARY)-$(VERSION)-$$os-$$arch$$ext"; \
		echo "  $$out"; \
		GOOS=$$os GOARCH=$$arch go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o "$$out" ./cmd/pawl || exit 1; \
	done
	@cp LICENSE.txt THIRD-PARTY-NOTICES.txt dist/
	@cd dist && sha256sum * > SHA256SUMS
	@echo "checksums:" && cat dist/SHA256SUMS
	@$(MAKE) --no-print-directory verify-dist

.PHONY: verify-dist
verify-dist: ## PAWL-011 AC9 — an artifact must report the version its name claims
	@host="dist/$(BINARY)-$(VERSION)-$$(go env GOOS)-$$(go env GOARCH)"; \
	if [ ! -x "$$host" ]; then \
		echo "verify-dist: no host-native artifact at $$host"; exit 1; \
	fi; \
	got="$$("$$host" version | awk '{print $$2}')"; \
	if [ "$$got" != "$(VERSION)" ]; then \
		echo "verify-dist: FAIL — $$host reports '$$got', filename claims '$(VERSION)'"; \
		exit 1; \
	fi; \
	echo "verify-dist: ok — $$host reports $$got"
	@echo "verify-dist: note — only the host-native artifact can be executed here."
	@echo "             Verifying the whole matrix needs a runner per platform and"
	@echo "             belongs in the release workflow (PAWL-013 AC9)."

.PHONY: clean
clean:
	rm -rf bin dist

.PHONY: help
help: ## List targets
	@grep -E '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-10s %s\n", $$1, $$2}'

check-pins: ## PAWL-025 AC1 — every GitHub Action pinned to an immutable commit
	@refs=$$(grep -h 'uses:' .github/workflows/*.yml \
		| sed 's/.*uses:[[:space:]]*//; s/[[:space:]]*#.*//; s/[[:space:]]*$$//'); \
	bad=$$(printf '%s\n' "$$refs" | grep -vE '@[0-9a-f]{40}$$' || true); \
	if [ -n "$$bad" ]; then \
		printf '%s\n' "$$bad" | sed 's/^/  UNPINNED  /'; \
		echo "check-pins: FAIL — a tag is a pointer its owner can move, and the"; \
		echo "            release job holds id-token: write."; \
		exit 1; \
	fi; \
	echo "check-pins: ok — $$(printf '%s\n' "$$refs" | grep -c . ) actions pinned to commits"
.PHONY: check-pins

fuzz: ## PAWL-025 AC8 — exercise the parsers that read input we did not produce
	go test -run=XXX -fuzz=FuzzJUnit      -fuzztime=$(FUZZTIME) ./internal/evidence
	go test -run=XXX -fuzz=FuzzCoverage   -fuzztime=$(FUZZTIME) ./internal/evidence
	go test -run=XXX -fuzz=FuzzTypecheck  -fuzztime=$(FUZZTIME) ./internal/evidence
	go test -run=XXX -fuzz=FuzzRecord     -fuzztime=$(FUZZTIME) ./internal/claimlog
.PHONY: fuzz

hooks: ## Point git at .githooks (PAWL-027) — one command per clone
	@git config core.hooksPath .githooks
	@echo "core.hooksPath = .githooks"
	@echo "commit-msg validates the message; pre-push runs make check."
	@echo "Both are bypassable with --no-verify; CI is the enforcement."
.PHONY: hooks

check-tagger: ## Lint: no workflow writes a tag directly
	@# A lint, and only a lint. It greps for a git invocation reaching `tag`,
	@# which catches every realistic form including `git -c k=v tag` and an
	@# absolute path — but a grep is not a shell parser and never will be, so
	@# indirection like `g=git; $$g tag` passes it.
	@#
	@# What actually guarantees the behaviour is TestTagScript* in internal/e2e:
	@# real git, a real bare remote, identity unset. An earlier version of this
	@# target also grepped tag.sh for `git config user.name`, which reported ok
	@# when both lines were commented out — a false assurance standing beside a
	@# real one. It was removed rather than tightened, because the test already
	@# establishes the property and this cannot.
	@bad=$$(grep -lE 'git[^;|&]*[[:space:]]tag([[:space:]]|$$)' \
		.github/workflows/*.yml .github/workflows/*.yaml 2>/dev/null || true); \
	if [ -n "$$bad" ]; then \
		printf '%s\n' "$$bad" | sed 's|^|  writes a tag directly: |'; \
		echo "check-tagger: FAIL — tags are written by .github/scripts/tag.sh only,"; \
		echo "              which is the one place that configures a tagger."; \
		exit 1; \
	fi; \
	[ -x .github/scripts/tag.sh ] || { echo "check-tagger: FAIL — .github/scripts/tag.sh is missing or not executable"; exit 1; }; \
	echo "check-tagger: ok — no workflow writes a tag directly (lint; behaviour covered by TestTagScript*)"
.PHONY: check-tagger
