.PHONY: build test lint install clean

build:
	go build -o bin/email-bison-pp-cli ./cmd/email-bison-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/email-bison-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/email-bison-pp-mcp ./cmd/email-bison-pp-mcp

install-mcp:
	go install ./cmd/email-bison-pp-mcp

build-all: build build-mcp
