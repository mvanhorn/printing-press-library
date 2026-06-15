.PHONY: build test lint install clean

build:
	go build -o bin/yeswehack-pp-cli ./cmd/yeswehack-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/yeswehack-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/yeswehack-pp-mcp ./cmd/yeswehack-pp-mcp

install-mcp:
	go install ./cmd/yeswehack-pp-mcp

build-all: build build-mcp
