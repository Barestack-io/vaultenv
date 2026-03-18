BINARY    := vaultenv
MODULE    := github.com/scaler/vaultenv
CMD       := ./cmd/vaultenv
BUILD_DIR := build

VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE      ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS   := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

# ─── Default ────────────────────────────────────────────────────

.PHONY: build
build: ## Build for the current platform
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

# ─── Cross-compilation ─────────────────────────────────────────

.PHONY: build-all
build-all: $(PLATFORMS) ## Build for all supported platforms

.PHONY: $(PLATFORMS)
$(PLATFORMS):
	$(eval GOOS   := $(word 1,$(subst /, ,$@)))
	$(eval GOARCH := $(word 2,$(subst /, ,$@)))
	$(eval EXT    := $(if $(filter windows,$(GOOS)),.exe,))
	@mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY)-$(GOOS)-$(GOARCH)$(EXT) $(CMD)
	@echo "  built $(BUILD_DIR)/$(BINARY)-$(GOOS)-$(GOARCH)$(EXT)"

.PHONY: build-for
build-for: ## Build for a specific platform: make build-for GOOS=linux GOARCH=amd64
ifndef GOOS
	$(error GOOS is required, e.g. make build-for GOOS=linux GOARCH=amd64)
endif
ifndef GOARCH
	$(error GOARCH is required, e.g. make build-for GOOS=linux GOARCH=amd64)
endif
	$(eval EXT := $(if $(filter windows,$(GOOS)),.exe,))
	@mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY)-$(GOOS)-$(GOARCH)$(EXT) $(CMD)
	@echo "  built $(BUILD_DIR)/$(BINARY)-$(GOOS)-$(GOARCH)$(EXT)"

# ─── Tests ──────────────────────────────────────────────────────

.PHONY: test
test: ## Run all tests
	go test ./... -count=1

.PHONY: test-v
test-v: ## Run all tests with verbose output
	go test ./... -v -count=1

TESTABLE_PKGS := ./internal/config ./internal/crypto ./internal/gitutil ./internal/storage ./internal/vault

.PHONY: test-cover
test-cover: ## Run tests with coverage report
	go test $(TESTABLE_PKGS) -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@echo "\nHTML report: go tool cover -html=coverage.out"

.PHONY: test-race
test-race: ## Run tests with race detector
	go test ./... -race -count=1

# ─── Quality ────────────────────────────────────────────────────

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: fmt
fmt: ## Format all Go files
	gofmt -s -w .

.PHONY: check
check: vet test ## Run vet + tests (CI gate)

# ─── Housekeeping ───────────────────────────────────────────────

.PHONY: clean
clean: ## Remove build artifacts
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

.PHONY: tidy
tidy: ## Tidy go modules
	go mod tidy

.PHONY: install
install: build ## Install to $GOPATH/bin
	go install -ldflags "$(LDFLAGS)" $(CMD)

# ─── Help ───────────────────────────────────────────────────────

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
