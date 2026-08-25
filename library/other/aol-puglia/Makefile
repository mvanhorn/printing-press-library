.PHONY: build test lint install clean

build:
	go build -o bin/aol-puglia-pp-cli ./cmd/aol-puglia-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/aol-puglia-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/aol-puglia-pp-mcp ./cmd/aol-puglia-pp-mcp

install-mcp:
	go install ./cmd/aol-puglia-pp-mcp

build-all: build build-mcp
