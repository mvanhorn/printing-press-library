.PHONY: build test lint install clean

build:
	go build -o bin/hayward-omnilogic-pp-cli ./cmd/hayward-omnilogic-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/hayward-omnilogic-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/hayward-omnilogic-pp-mcp ./cmd/hayward-omnilogic-pp-mcp

install-mcp:
	go install ./cmd/hayward-omnilogic-pp-mcp

build-all: build build-mcp
