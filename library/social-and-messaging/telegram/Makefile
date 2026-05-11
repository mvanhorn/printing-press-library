.PHONY: build test lint install clean

build:
	go build -o bin/telegram-pp-cli ./cmd/telegram-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/telegram-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/telegram-pp-mcp ./cmd/telegram-pp-mcp

install-mcp:
	go install ./cmd/telegram-pp-mcp

build-all: build build-mcp
