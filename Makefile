GO ?= go
GOLANGCI_LINT ?= golangci-lint
BINARY := kkachi-agent-tester
BIN_DIR := bin
VERSION ?= 0.1.3
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
TOOLCHAIN_ROOT ?= $(HOME)/.local/kkachi/toolchains
TOOLCHAIN_COMPONENT := kat
TOOLCHAIN_VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null | sed 's/^v//' || true)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

UNIT_PACKAGES := ./internal/artifacts ./internal/config ./internal/extract ./internal/rules ./internal/runner ./internal/safety
INTEGRATION_PACKAGES := ./internal/cli
E2E_PACKAGES := ./e2e

.PHONY: build install install-toolchain test format lint vet guardrails unit-test integration-test e2e-test clean

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./cmd/$(BINARY)

install:
	env -u GOPATH $(GO) install -ldflags "$(LDFLAGS)" ./cmd/$(BINARY)

install-toolchain:
	@set -e; \
	VERSION_VALUE="$(VERSION)"; \
	if [ "$$VERSION_VALUE" = "0.0.0-dev" ]; then VERSION_VALUE="$(TOOLCHAIN_VERSION)"; fi; \
	VERSION_VALUE="$${VERSION_VALUE#v}"; \
	if [ -z "$$VERSION_VALUE" ] || [ "$$VERSION_VALUE" = "0.0.0-dev" ]; then \
		echo "ERROR: install-toolchain requires a real version; set VERSION=0.1.x or run from an exact v0.1.x tag" >&2; \
		exit 1; \
	fi; \
	$(MAKE) build VERSION="$$VERSION_VALUE"; \
	VERSION_TAG="v$$VERSION_VALUE"; \
	case "$$VERSION_TAG" in v[0-9]*.[0-9]*.[0-9]*) ;; *) echo "ERROR: unsupported $(BINARY) version for toolchain install: $$VERSION_VALUE" >&2; exit 1 ;; esac; \
	INSTALL_DIR="$(TOOLCHAIN_ROOT)/$(TOOLCHAIN_COMPONENT)/$$VERSION_TAG/bin"; \
	mkdir -p "$$INSTALL_DIR"; \
	install -m 0755 "$(BIN_DIR)/$(BINARY)" "$$INSTALL_DIR/$(BINARY)"; \
	INSTALLED_VERSION="$$($$INSTALL_DIR/$(BINARY) --version | awk '{print $$2}')"; \
	if [ "$${INSTALLED_VERSION#v}" != "$$VERSION_VALUE" ]; then \
		echo "ERROR: installed version mismatch: expected $$VERSION_VALUE, got $$INSTALLED_VERSION" >&2; \
		exit 1; \
	fi; \
	echo "installed $(BINARY) $$VERSION_TAG to $$INSTALL_DIR/$(BINARY)"

test:
	$(MAKE) format
	$(MAKE) lint
	$(MAKE) vet
	$(MAKE) guardrails
	$(MAKE) unit-test
	$(MAKE) integration-test
	$(MAKE) e2e-test

format:
	$(GO) fmt ./...

lint:
	$(GOLANGCI_LINT) run ./...

vet:
	$(GO) vet ./...

guardrails:
	$(GO) test -count=1 ./internal/config -run '^TestValidateRejectsUnknownParser$$'
	$(GO) test -count=1 ./internal/extract -run '^TestProcessExtractorStatusContract$$'
	$(GO) test -count=1 ./internal/rules -run '^(TestLoadApplicableFailsOnInvalidDiscoveredFutureParserRule|TestLoadApplicableFailsOnInvalidMatchingRule|TestRuleDetectsOvermatch)$$'
	$(GO) test -count=1 ./internal/cli -run '^(TestMaterializeArtifactsExtractionErrorRetainsNonPassRunState|TestRawLogPersistsWhenExtractionFails|TestRunInternalErrorAfterPassedCommandMaterializesArtifacts|TestSummarizeInternalErrorMaterializesArtifacts|TestSummarizeRebuildsArtifactsFromRawLogOnly|TestRulesLifecycleCommands)$$'

unit-test:
	$(GO) test -count=1 $(UNIT_PACKAGES)

integration-test:
	$(GO) test -count=1 $(INTEGRATION_PACKAGES)

e2e-test:
	$(GO) test -count=1 $(E2E_PACKAGES)

clean:
	rm -rf $(BIN_DIR)
