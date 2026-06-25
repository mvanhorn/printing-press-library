.PHONY: build test lint install clean

build:
	go build -o bin/rechtspraak-pp-cli ./cmd/rechtspraak-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/rechtspraak-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/rechtspraak-pp-mcp ./cmd/rechtspraak-pp-mcp

install-mcp:
	go install ./cmd/rechtspraak-pp-mcp

build-all: build build-mcp
