.PHONY: build test lint install clean

build:
	go build -o bin/authentik-pp-cli ./cmd/authentik-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/authentik-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/authentik-pp-mcp ./cmd/authentik-pp-mcp

install-mcp:
	go install ./cmd/authentik-pp-mcp

build-all: build build-mcp
