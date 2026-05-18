.PHONY: build test lint install clean

build:
	go build -o bin/tella-pp-cli ./cmd/tella-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tella-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/tella-pp-mcp ./cmd/tella-pp-mcp

install-mcp:
	go install ./cmd/tella-pp-mcp

build-all: build build-mcp
