.PHONY: build test lint install clean

build:
	go build -o bin/cisa-kev-pp-cli ./cmd/cisa-kev-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/cisa-kev-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/cisa-kev-pp-mcp ./cmd/cisa-kev-pp-mcp

install-mcp:
	go install ./cmd/cisa-kev-pp-mcp

build-all: build build-mcp
