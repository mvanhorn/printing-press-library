.PHONY: build test lint install clean

build:
	go build -o bin/boardgamegeek-pp-cli ./cmd/boardgamegeek-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/boardgamegeek-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/boardgamegeek-pp-mcp ./cmd/boardgamegeek-pp-mcp

install-mcp:
	go install ./cmd/boardgamegeek-pp-mcp

build-all: build build-mcp
