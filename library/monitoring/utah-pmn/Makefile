.PHONY: build test lint install clean

build:
	go build -o bin/utah-pmn-pp-cli ./cmd/utah-pmn-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/utah-pmn-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/utah-pmn-pp-mcp ./cmd/utah-pmn-pp-mcp

install-mcp:
	go install ./cmd/utah-pmn-pp-mcp

build-all: build build-mcp
