.PHONY: build test lint install clean

build:
	go build -o bin/trello-pp-cli ./cmd/trello-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/trello-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/trello-pp-mcp ./cmd/trello-pp-mcp

install-mcp:
	go install ./cmd/trello-pp-mcp

build-all: build build-mcp
