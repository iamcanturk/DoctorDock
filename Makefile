BINARY      := doctordock
ALIAS       := ddock
MODULE      := github.com/iamcanturk/DoctorDock
BIN_DIR     := bin

# A tagged build reports the tag. Before the first tag exists, `git describe`
# would return a bare commit hash, which is not a version anyone can reason
# about, so it becomes a dev build of the version being worked towards.
NEXT_VERSION := 0.1.0
VERSION     ?= $(shell git describe --tags --exact-match 2>/dev/null \
                 || git describe --tags --dirty 2>/dev/null \
                 || echo "$(NEXT_VERSION)-dev+$$(git rev-parse --short HEAD 2>/dev/null || echo unknown)")
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

# -s -w strip the symbol table and DWARF data; the Docker SDK makes the binary
# large enough that this is worth doing. -trimpath keeps build paths out of it.
LDFLAGS     := -s -w \
               -X main.version=$(VERSION) \
               -X main.commit=$(COMMIT)
GOFLAGS     := -trimpath

.DEFAULT_GOAL := build

.PHONY: build
build: ## Build the binary into bin/
	@mkdir -p $(BIN_DIR)
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./cmd/doctordock
	@cp $(BIN_DIR)/$(BINARY) $(BIN_DIR)/$(ALIAS)
	@echo "built $(BIN_DIR)/$(BINARY) ($(VERSION))"

# Where `make install` puts the binary. The first writable directory that is
# already on PATH is the right answer on most machines; override with
# `make install PREFIX=/somewhere`.
PREFIX      ?= $(shell for d in /opt/homebrew/bin /usr/local/bin $$HOME/.local/bin; do \
                 if [ -w "$$d" ]; then echo "$$d"; break; fi; done)

.PHONY: install
install: build ## Install doctordock and the ddock alias onto PATH
	@if [ -z "$(PREFIX)" ]; then \
		echo "no writable install directory found; try: make install PREFIX=~/.local/bin"; \
		exit 1; \
	fi
	@mkdir -p "$(PREFIX)"
	install -m 0755 $(BIN_DIR)/$(BINARY) "$(PREFIX)/$(BINARY)"
	@ln -sf "$(PREFIX)/$(BINARY)" "$(PREFIX)/$(ALIAS)"
	@echo "installed $(PREFIX)/$(BINARY) and $(PREFIX)/$(ALIAS) ($(VERSION))"

.PHONY: uninstall
uninstall: ## Remove the installed binary and alias
	rm -f "$(PREFIX)/$(BINARY)" "$(PREFIX)/$(ALIAS)"
	@echo "removed from $(PREFIX)"

.PHONY: completions
completions: build ## Install shell completion for the current shell
	@./scripts/install-completions.sh "$(BIN_DIR)/$(BINARY)"

.PHONY: go-install
go-install: ## Install into GOPATH/bin with `go install`
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)" ./cmd/doctordock

.PHONY: run
run: build ## Build and run a scan
	@$(BIN_DIR)/$(BINARY)

.PHONY: test
test: ## Run unit tests (no Docker daemon required)
	go test ./...

.PHONY: test-race
test-race: ## Run unit tests with the race detector
	go test -race ./...

.PHONY: test-integration
test-integration: ## Run integration tests (requires a Docker daemon)
	go test -tags integration -v ./tests/integration/...

.PHONY: test-e2e
test-e2e: ## Rule coverage against a real daemon (creates and removes ddtest-* resources)
	./tests/e2e/run.sh

.PHONY: cover
cover: ## Run tests and open the coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: fmt
fmt: ## Format the source
	gofmt -w .

.PHONY: fmt-check
fmt-check: ## Fail if anything is unformatted
	@files=$$(gofmt -l . 2>/dev/null); \
	if [ -n "$$files" ]; then echo "unformatted files:"; echo "$$files"; exit 1; fi

.PHONY: docs
docs: ## Regenerate docs/RULES.md from the rule registry
	go run ./tools/gendocs
	@echo "regenerated docs/RULES.md"

.PHONY: docs-check
docs-check: ## Fail if docs/RULES.md is out of date
	@go run ./tools/gendocs
	@git diff --exit-code docs/RULES.md \
		|| (echo "docs/RULES.md is out of date; run 'make docs'"; exit 1)

.PHONY: cross
cross: ## Build every release target, to catch platform-specific breakage
	@for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do \
		GOOS=$${t%/*} GOARCH=$${t#*/} go build -o /dev/null ./... \
			&& echo "  ok    $$t" \
			|| { echo "  FAIL  $$t"; exit 1; }; \
	done

.PHONY: check
check: fmt-check vet build test ## Fast pre-commit checks

.PHONY: verify
verify: ## Everything, locally. Run this before tagging a release.
	@./scripts/verify.sh

.PHONY: lint
lint: ## Run golangci-lint if it is installed
	@command -v golangci-lint >/dev/null 2>&1 \
		&& golangci-lint run ./... \
		|| echo "golangci-lint not installed; skipping"

.PHONY: snapshot
snapshot: ## Build release artifacts locally without publishing
	goreleaser release --snapshot --clean

.PHONY: clean
clean: ## Remove build output
	rm -rf $(BIN_DIR) dist coverage.out

.PHONY: help
help: ## List targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
