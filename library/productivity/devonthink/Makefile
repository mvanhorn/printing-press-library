.PHONY: build test lint install clean

build:
	go build -o bin/devonthink-pp-cli ./cmd/devonthink-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/devonthink-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/devonthink-pp-mcp ./cmd/devonthink-pp-mcp

install-mcp:
	go install ./cmd/devonthink-pp-mcp

build-all: build build-mcp
