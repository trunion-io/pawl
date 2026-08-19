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
GOFLAGS := -trimpath
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
check: fmt vet test check-pins ## What CI runs

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

fuzz: ## PAWL-025 AC8 — exercise the parsers that read input we did not produce
	go test -run=XXX -fuzz=FuzzJUnit      -fuzztime=$(FUZZTIME) ./internal/evidence
	go test -run=XXX -fuzz=FuzzCoverage   -fuzztime=$(FUZZTIME) ./internal/evidence
	go test -run=XXX -fuzz=FuzzTypecheck  -fuzztime=$(FUZZTIME) ./internal/evidence
	go test -run=XXX -fuzz=FuzzRecord     -fuzztime=$(FUZZTIME) ./internal/claimlog
