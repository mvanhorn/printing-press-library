.PHONY: build test lint install clean

build:
	go build -o bin/rocket-league-tracker-pp-cli ./cmd/rocket-league-tracker-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/rocket-league-tracker-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/rocket-league-tracker-pp-mcp ./cmd/rocket-league-tracker-pp-mcp

install-mcp:
	go install ./cmd/rocket-league-tracker-pp-mcp

build-all: build build-mcp
