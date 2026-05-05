.PHONY: build build-readonly build-agent-safe test lint install clean build-mcp build-mcp-readonly build-mcp-agent-safe install-mcp build-all

build:
	go build -o bin/google-photos-pp-cli ./cmd/google-photos-pp-cli

build-readonly:
	go build -tags safety_readonly -o bin/google-photos-pp-cli-readonly ./cmd/google-photos-pp-cli

build-agent-safe:
	go build -tags safety_agent_safe -o bin/google-photos-pp-cli-agent-safe ./cmd/google-photos-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/google-photos-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/google-photos-pp-mcp ./cmd/google-photos-pp-mcp

build-mcp-readonly:
	go build -tags safety_readonly -o bin/google-photos-pp-mcp-readonly ./cmd/google-photos-pp-mcp

build-mcp-agent-safe:
	go build -tags safety_agent_safe -o bin/google-photos-pp-mcp-agent-safe ./cmd/google-photos-pp-mcp

install-mcp:
	go install ./cmd/google-photos-pp-mcp

build-all: build build-mcp
