.PHONY: build test lint install clean

build:
	go build -o bin/drudgereport-pp-cli ./cmd/drudgereport-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/drudgereport-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/drudgereport-pp-mcp ./cmd/drudgereport-pp-mcp

install-mcp:
	go install ./cmd/drudgereport-pp-mcp

build-all: build build-mcp
