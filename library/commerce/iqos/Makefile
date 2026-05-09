.PHONY: build test lint install clean

build:
	go build -o bin/iqos-pp-cli ./cmd/iqos-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/iqos-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/iqos-pp-mcp ./cmd/iqos-pp-mcp

install-mcp:
	go install ./cmd/iqos-pp-mcp

build-all: build build-mcp
