.PHONY: build test lint install clean

build:
	go build -o bin/resend-pp-cli ./cmd/resend-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/resend-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/resend-pp-mcp ./cmd/resend-pp-mcp

install-mcp:
	go install ./cmd/resend-pp-mcp

build-all: build build-mcp
