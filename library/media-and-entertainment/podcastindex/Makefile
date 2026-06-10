.PHONY: build test lint install clean

build:
	go build -o bin/podcastindex-pp-cli ./cmd/podcastindex-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/podcastindex-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/podcastindex-pp-mcp ./cmd/podcastindex-pp-mcp

install-mcp:
	go install ./cmd/podcastindex-pp-mcp

build-all: build build-mcp
