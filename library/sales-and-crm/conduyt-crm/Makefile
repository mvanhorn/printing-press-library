.PHONY: build test lint install clean sync-spec

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/conduyt-crm-pp-cli$(BIN_EXT) ./cmd/conduyt-crm-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/conduyt-crm-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/conduyt-crm-pp-mcp$(BIN_EXT) ./cmd/conduyt-crm-pp-mcp

install-mcp:
	go install ./cmd/conduyt-crm-pp-mcp

build-all: build build-mcp

# (Conduyt audit CND-020) refresh the public API document the drift gate compares spec.yaml against
sync-spec:
	curl -fsSL https://conduyt.app/openapi.yaml -o upstream/openapi.yaml
	@echo "upstream/openapi.yaml refreshed; run 'go test ./internal/cli -run TestSpecMatchesUpstreamPublicAPI'"
