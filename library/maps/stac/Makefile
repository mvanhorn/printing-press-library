.PHONY: build test lint install clean

build:
	go build -o bin/stac-pp-cli ./cmd/stac-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/stac-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/stac-pp-mcp ./cmd/stac-pp-mcp

install-mcp:
	go install ./cmd/stac-pp-mcp

build-all: build build-mcp
