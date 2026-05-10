.PHONY: build test lint install clean

build:
	go build -o bin/thevc-pp-cli ./cmd/thevc-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/thevc-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/thevc-pp-mcp ./cmd/thevc-pp-mcp

install-mcp:
	go install ./cmd/thevc-pp-mcp

build-all: build build-mcp
