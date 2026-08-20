.PHONY: build test lint install clean

build:
	go build -o bin/opensnow-pp-cli ./cmd/opensnow-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/opensnow-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/opensnow-pp-mcp ./cmd/opensnow-pp-mcp

install-mcp:
	go install ./cmd/opensnow-pp-mcp

build-all: build build-mcp
