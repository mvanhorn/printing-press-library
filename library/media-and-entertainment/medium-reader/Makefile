.PHONY: build test lint install clean build-chrome

build:
	go build -o bin/medium-reader-pp-cli ./cmd/medium-reader-pp-cli

# Opt-in build with real `auth login --chrome` Chrome cookie extraction via
# kooky. NOT the default/shipped build: it needs CGO + a native toolchain (kooky
# decrypts Chrome's keychain-protected cookie store) and triggers a macOS
# Keychain prompt on first use, so it is not pure-Go cross-compilable. kooky is a
# build-tag-only dependency — do NOT run a plain `go mod tidy`, which would strip
# it; tidy with the tag (e.g. GOFLAGS=-tags=kooky) or leave go.mod as generated.
build-chrome:
	CGO_ENABLED=1 go build -tags kooky -o bin/medium-reader-pp-cli ./cmd/medium-reader-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/medium-reader-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/medium-reader-pp-mcp ./cmd/medium-reader-pp-mcp

install-mcp:
	go install ./cmd/medium-reader-pp-mcp

build-all: build build-mcp
