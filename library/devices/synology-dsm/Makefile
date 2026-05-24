.PHONY: build test lint install clean

build:
	go build -o bin/synology-dsm-pp-cli ./cmd/synology-dsm-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/synology-dsm-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/synology-dsm-pp-mcp ./cmd/synology-dsm-pp-mcp

install-mcp:
	go install ./cmd/synology-dsm-pp-mcp

build-all: build build-mcp
