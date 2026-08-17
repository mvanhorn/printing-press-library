.PHONY: build test lint install clean

build:
	go build -o bin/ikon-pp-cli ./cmd/ikon-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ikon-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ikon-pp-mcp ./cmd/ikon-pp-mcp

install-mcp:
	go install ./cmd/ikon-pp-mcp

build-all: build build-mcp
