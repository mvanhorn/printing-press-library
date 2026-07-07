.PHONY: build test lint install clean

build:
	go build -o bin/lda-gov-pp-cli ./cmd/lda-gov-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/lda-gov-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/lda-gov-pp-mcp ./cmd/lda-gov-pp-mcp

install-mcp:
	go install ./cmd/lda-gov-pp-mcp

build-all: build build-mcp
