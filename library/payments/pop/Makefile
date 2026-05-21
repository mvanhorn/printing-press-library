.PHONY: build test lint install clean

build:
	go build -o bin/pop-pp-cli ./cmd/pop-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pop-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pop-pp-mcp ./cmd/pop-pp-mcp

install-mcp:
	go install ./cmd/pop-pp-mcp

build-all: build build-mcp
