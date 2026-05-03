.PHONY: build test lint install clean

build:
	go build -o bin/pagliacci-pp-cli ./cmd/pagliacci-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pagliacci-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pagliacci-pp-mcp ./cmd/pagliacci-pp-mcp

install-mcp:
	go install ./cmd/pagliacci-pp-mcp

build-all: build build-mcp
