.PHONY: build test lint install clean

build:
	go build -o bin/loopnet-pp-cli ./cmd/loopnet-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/loopnet-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/loopnet-pp-mcp ./cmd/loopnet-pp-mcp

install-mcp:
	go install ./cmd/loopnet-pp-mcp

build-all: build build-mcp
