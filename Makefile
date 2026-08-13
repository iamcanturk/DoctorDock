BINARY      := doctordock
ALIAS       := ddock
MODULE      := github.com/iamcanturk/DoctorDock
BIN_DIR     := bin

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
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

.PHONY: install
install: ## Install the binary into GOPATH/bin
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

.PHONY: check
check: fmt-check vet build test ## Everything CI runs

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
