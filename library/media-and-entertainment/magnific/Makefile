.PHONY: build test lint install clean

build:
	go build -o bin/magnific-pp-cli ./cmd/magnific-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/magnific-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/magnific-pp-mcp ./cmd/magnific-pp-mcp

install-mcp:
	go install ./cmd/magnific-pp-mcp

build-all: build build-mcp
