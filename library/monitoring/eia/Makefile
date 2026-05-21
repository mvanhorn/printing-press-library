.PHONY: build test lint install clean

build:
	go build -o bin/eia-pp-cli ./cmd/eia-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/eia-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/eia-pp-mcp ./cmd/eia-pp-mcp

install-mcp:
	go install ./cmd/eia-pp-mcp

build-all: build build-mcp
