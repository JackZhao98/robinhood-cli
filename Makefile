# robinhood-cli Makefile — build/install/release with stamped versions.
#
# Why this exists: `go build` produces binaries with main.version="dev",
# making it impossible to tell *which* rh you're running. This injects
# `git describe --tags --dirty` + commit short SHA + UTC timestamp via
# -ldflags so `rh version` always tells the truth.

SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# --- knobs ---------------------------------------------------------------

BIN_DIR     ?= bin
INSTALL_DIR ?= $(HOME)/.local/bin
BIN_NAME    ?= rh

# Version info, derived from git. Examples:
#   on a tag       : v1.2.0
#   ahead of tag   : v1.2.0-3-gabc1234
#   dirty tree     : v1.2.0-3-gabc1234-dirty
#   no tags at all : 0.0.0-<sha>
VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo "0.0.0")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILT   ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X main.version=$(VERSION) \
  -X main.commit=$(COMMIT) \
  -X main.builtAt=$(BUILT)

# --- targets -------------------------------------------------------------

.PHONY: help
help:  ## Show available targets
	@awk -F'## ' '/^[a-z][a-zA-Z_-]+:.*## / { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST) \
	  | sed 's/:[^ ]*//'

.PHONY: version
version:  ## Show the version that would be stamped right now
	@echo "VERSION=$(VERSION)"
	@echo "COMMIT=$(COMMIT)"
	@echo "BUILT=$(BUILT)"

.PHONY: build
build:  ## Build $(BIN_DIR)/$(BIN_NAME) with stamped version metadata
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BIN_NAME) ./cmd/$(BIN_NAME)
	@echo "ok built $(BIN_DIR)/$(BIN_NAME) ($(VERSION))"

.PHONY: install
install: build  ## Build + copy binary to $(INSTALL_DIR) (overwrites existing rh)
	@mkdir -p $(INSTALL_DIR)
	@if [ -f $(INSTALL_DIR)/$(BIN_NAME) ]; then \
	  cp -p $(INSTALL_DIR)/$(BIN_NAME) $(INSTALL_DIR)/$(BIN_NAME).bak; \
	  echo "→ backed up previous $(BIN_NAME) → $(BIN_NAME).bak"; \
	fi
	cp $(BIN_DIR)/$(BIN_NAME) $(INSTALL_DIR)/$(BIN_NAME)
	chmod +x $(INSTALL_DIR)/$(BIN_NAME)
	@echo "ok installed $(INSTALL_DIR)/$(BIN_NAME) ($(VERSION))"
	@$(INSTALL_DIR)/$(BIN_NAME) version

.PHONY: test
test:  ## Run go test ./... with race detector
	go test -race ./...

.PHONY: vet
vet:  ## go vet ./...
	go vet ./...

.PHONY: pre-release-check
pre-release-check:  ## Refuse to release with uncommitted/unpushed changes
	@if [ -n "$$(git status --porcelain)" ]; then \
	  echo "✗ working tree dirty — commit/stash before releasing"; \
	  git status --short; \
	  exit 1; \
	fi
	@git fetch origin --quiet
	@LOCAL=$$(git rev-parse @); \
	REMOTE=$$(git rev-parse @{u} 2>/dev/null || echo ""); \
	if [ "$$LOCAL" != "$$REMOTE" ]; then \
	  echo "✗ local HEAD ≠ origin — push first"; \
	  exit 1; \
	fi

.PHONY: release
release: pre-release-check test  ## Tag + push a release. Pass NEW_VERSION=v1.2.1
	@if [ -z "$(NEW_VERSION)" ]; then \
	  echo "✗ NEW_VERSION required, e.g. make release NEW_VERSION=v1.2.1"; \
	  echo "  (current latest tag: $$(git describe --tags --abbrev=0 2>/dev/null || echo none))"; \
	  exit 1; \
	fi
	@if ! echo "$(NEW_VERSION)" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+(-.+)?$$'; then \
	  echo "✗ NEW_VERSION '$(NEW_VERSION)' must look like vMAJOR.MINOR.PATCH (semver, with leading v)"; \
	  exit 1; \
	fi
	@if git rev-parse "$(NEW_VERSION)" >/dev/null 2>&1; then \
	  echo "✗ tag $(NEW_VERSION) already exists"; \
	  exit 1; \
	fi
	git tag -a $(NEW_VERSION) -m "Release $(NEW_VERSION)"
	git push origin $(NEW_VERSION)
	@echo "ok released $(NEW_VERSION)"
	@echo "→ run 'make install' to put the tagged binary in $(INSTALL_DIR)"

.PHONY: clean
clean:  ## Remove $(BIN_DIR)
	@rm -rf $(BIN_DIR)
	@echo "ok cleaned $(BIN_DIR)/"
