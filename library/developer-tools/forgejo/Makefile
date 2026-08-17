.PHONY: build test lint install clean

build:
	go build -o bin/fj ./cmd/fj
	go build -o bin/forgejo-pp-cli ./cmd/forgejo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/fj
	go install ./cmd/forgejo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/forgejo-pp-mcp ./cmd/forgejo-pp-mcp

install-mcp:
	go install ./cmd/forgejo-pp-mcp

build-all: build build-mcp
