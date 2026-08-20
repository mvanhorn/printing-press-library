.PHONY: build test lint install clean

build:
	go build -o bin/roam-pp-cli ./cmd/roam-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/roam-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/roam-pp-mcp ./cmd/roam-pp-mcp

install-mcp:
	go install ./cmd/roam-pp-mcp

build-all: build build-mcp
