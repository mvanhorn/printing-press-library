.PHONY: build test lint install clean

build:
	go build -o bin/tenderned-pp-cli ./cmd/tenderned-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tenderned-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/tenderned-pp-mcp ./cmd/tenderned-pp-mcp

install-mcp:
	go install ./cmd/tenderned-pp-mcp

build-all: build build-mcp
