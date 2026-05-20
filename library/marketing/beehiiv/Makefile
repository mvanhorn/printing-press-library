.PHONY: build test lint install clean

build:
	go build -o bin/beehiiv-pp-cli ./cmd/beehiiv-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/beehiiv-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/beehiiv-pp-mcp ./cmd/beehiiv-pp-mcp

install-mcp:
	go install ./cmd/beehiiv-pp-mcp

build-all: build build-mcp
