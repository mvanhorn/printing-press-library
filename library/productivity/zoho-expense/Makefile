.PHONY: build test lint install clean

build:
	go build -o bin/zoho-expense-pp-cli ./cmd/zoho-expense-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zoho-expense-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zoho-expense-pp-mcp ./cmd/zoho-expense-pp-mcp

install-mcp:
	go install ./cmd/zoho-expense-pp-mcp

build-all: build build-mcp
