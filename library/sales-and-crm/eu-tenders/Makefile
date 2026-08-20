.PHONY: build test lint install clean

build:
	go build -o bin/eu-tenders-pp-cli ./cmd/eu-tenders-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/eu-tenders-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/eu-tenders-pp-mcp ./cmd/eu-tenders-pp-mcp

install-mcp:
	go install ./cmd/eu-tenders-pp-mcp

build-all: build build-mcp
