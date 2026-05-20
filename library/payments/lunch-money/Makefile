.PHONY: build test lint install clean

build:
	go build -o bin/lunch-money-pp-cli ./cmd/lunch-money-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/lunch-money-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/lunch-money-pp-mcp ./cmd/lunch-money-pp-mcp

install-mcp:
	go install ./cmd/lunch-money-pp-mcp

build-all: build build-mcp
