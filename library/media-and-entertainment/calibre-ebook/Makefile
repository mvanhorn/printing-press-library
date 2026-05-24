.PHONY: build test lint install clean

build:
	go build -o bin/calibre-ebook-pp-cli ./cmd/calibre-ebook-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/calibre-ebook-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/calibre-ebook-pp-mcp ./cmd/calibre-ebook-pp-mcp

install-mcp:
	go install ./cmd/calibre-ebook-pp-mcp

build-all: build build-mcp
