.PHONY: build test lint install clean

build:
	go build -o bin/zaptec-pp-cli ./cmd/zaptec-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zaptec-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zaptec-pp-mcp ./cmd/zaptec-pp-mcp

install-mcp:
	go install ./cmd/zaptec-pp-mcp

build-all: build build-mcp
