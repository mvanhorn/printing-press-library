.PHONY: build test lint install clean

build:
	go build -o bin/smartlead-pp-cli ./cmd/smartlead-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/smartlead-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/smartlead-pp-mcp ./cmd/smartlead-pp-mcp

install-mcp:
	go install ./cmd/smartlead-pp-mcp

build-all: build build-mcp
