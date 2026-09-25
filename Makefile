GO ?= go
# VERSION reaches the binary through -ldflags. A build outside a tag falls back
# to the module system's answer, which is what go install writes.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null)
LDFLAGS := -X main.version=$(VERSION)
LINTER_VERSION ?= v2.13.1
VULNCHECK_VERSION ?= v1.7.0
DEADCODE_VERSION ?= v0.38.0
BIN ?= $(CURDIR)/bin

.DEFAULT_GOAL := check

.PHONY: ci-local build install cross test-race fmt vet check supply-chain fuzz quick tools cover-zero

build:
	@mkdir -p $(BIN)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN)/mirugit ./cmd/mirugit

install:
	$(GO) mod download

test-race:
	$(GO) test -race ./...

fmt:
	@set -e; \
	file_list=$$(mktemp "$${TMPDIR:-/tmp}/mirugit-gofmt.XXXXXX"); \
	trap 'rm -f "$$file_list"' 0; \
	find . -path './.git' -prune -o -path './.ci-cache' -prune -o -path './bin' -prune -o -type f -name '*.go' -print0 > "$$file_list"; \
	unformatted=$$(xargs -0 gofmt -l < "$$file_list") || exit $$?; \
	if [ -n "$$unformatted" ]; then \
		printf '%s\n' "$$unformatted"; \
		echo "run gofmt -w on the listed files"; \
		exit 1; \
	fi

vet:
	$(GO) vet ./...

tools:
	git config core.hooksPath .githooks
# FUZZTIME is per target. The seed corpus and every input a past run found are
# replayed by go test on its own; this is the search for new ones.
FUZZTIME ?= 20s
FUZZ_PKGS := ./internal/git ./internal/layout

fuzz:
	@set -e; \
	targets=$$(mktemp "$${TMPDIR:-/tmp}/mirugit-fuzz.XXXXXX"); \
	trap 'rm -f "$$targets"' 0; \
	for pkg in $(FUZZ_PKGS); do \
		listed=$$($(GO) test -list 'Fuzz.*' "$$pkg") || exit $$?; \
		found=0; \
		for t in $$listed; do \
			case "$$t" in \
				Fuzz*) printf '%s\0%s\0' "$$pkg" "$$t" >> "$$targets"; found=1 ;; \
			esac; \
		done; \
		if [ "$$found" -eq 0 ]; then \
			printf 'no fuzz tests found in %s\n' "$$pkg" >&2; \
			exit 1; \
		fi; \
	done; \
	xargs -0 -P 4 -n 2 sh -c ' \
		set -e; \
		pkg=$$0; target=$$1; \
		log=$$(mktemp "$${TMPDIR:-/tmp}/mirugit-fuzz-output.XXXXXX"); \
		trap "rm -f \"$$log\"" 0; \
		printf "== %s %s\n" "$$pkg" "$$target"; \
		if $(GO) test -parallel=1 -run "^$$" -fuzz "^$$target$$" -fuzztime=$(FUZZTIME) "$$pkg" > "$$log" 2>&1; then \
			sed "s|^|[$$pkg $$target] |" "$$log"; \
		else \
			status=$$?; \
			sed "s|^|[$$pkg $$target] |" "$$log" >&2; \
			exit "$$status"; \
		fi \
	' < "$$targets"

# Windows is refused at startup rather than at compile time, which only holds
# while the packages still build there.
cross:
	GOOS=windows GOARCH=amd64 $(GO) build -o /dev/null ./...
	GOOS=linux GOARCH=arm64 $(GO) build -o /dev/null ./...

supply-chain: build cross
	$(GO) mod tidy -diff
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(VULNCHECK_VERSION) ./...
	./tools/notices.sh $(BIN)/mirugit $(BIN)/THIRD_PARTY_NOTICES.txt
	@out=$$($(GO) run golang.org/x/tools/cmd/deadcode@$(DEADCODE_VERSION) -test ./... | grep -v '\.event$$'); \
	if [ -n "$$out" ]; then echo "$$out"; echo "unreachable code above"; exit 1; fi

CI_GATES := check test-race fuzz supply-chain

# The Linux gates run against this checkout through a container, so BIN points
# somewhere inside it: the tools are native binaries and the host's bin/ holds
# the ones this Mac uses. The two volumes keep the module and build caches
# between runs; without them every run recompiles the standard library.
#
# git needs an identity before internal/tui can commit, and it refuses a
# directory owned by another user without safe.directory. Both are what the ci
# workflow set.
LINUX_IMAGE ?= golang:1.26
CI_FUZZTIME ?= 30s

ci-local: export VERSION := $(VERSION)

ci-local:
	@mkdir -p "$(CURDIR)/.ci-cache/linux-gomodcache" "$(CURDIR)/.ci-cache/linux-sumdbcache" "$(CURDIR)/bin/linux-gobuildcache"
	@echo "== $(shell $(GO) env GOOS)/$(shell $(GO) env GOARCH)"
	$(MAKE) $(CI_GATES) FUZZTIME=$(CI_FUZZTIME)
	@echo "== linux ($(LINUX_IMAGE))"
	docker run --rm \
		--user "$(shell id -u):$(shell id -g)" \
		-e HOME=/tmp/mirugit-home \
		-e VERSION \
		-e GOPATH=/go \
		-e GOMODCACHE=/go/pkg/mod \
		-e GOCACHE=/go-cache \
		-v "$(CURDIR)":/src \
		-v "$(CURDIR)/.ci-cache/linux-gomodcache":/go/pkg/mod \
		-v "$(CURDIR)/.ci-cache/linux-sumdbcache":/go/pkg/sumdb \
		-v "$(CURDIR)/bin/linux-gobuildcache":/go-cache \
		-w /src $(LINUX_IMAGE) sh -c '\
			mkdir -p "$$HOME" && \
			git -C "$$HOME" config --global user.email ci@example.com && \
			git -C "$$HOME" config --global user.name CI && \
			git -C "$$HOME" config --global --add safe.directory /src && \
			make BIN=/tmp/mirugit-bin $(CI_GATES) FUZZTIME=$(CI_FUZZTIME)'

check: fmt vet
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINTER_VERSION) run
	$(GO) test ./...

# The same gates as check, minus the tests that shell out to git. Run check
# before pushing; CI runs the full set.
quick: fmt vet
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINTER_VERSION) run
	$(GO) test -short ./...

# cover-zero names every function no test reaches. Each one is either worth a
# test or worth deleting; the answer for the ones that are neither is written
# next to them in CONTRIBUTING.md.
cover-zero:
	$(GO) test -count=1 -coverpkg=./... -coverprofile=cover.out ./...
	$(GO) tool cover -func=cover.out | tail -1
	@$(GO) tool cover -func=cover.out | awk '$$3=="0.0%"'
