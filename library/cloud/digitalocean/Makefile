.PHONY: build test lint install clean

build:
	go build -o bin/digitalocean-pp-cli ./cmd/digitalocean-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/digitalocean-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/digitalocean-pp-mcp ./cmd/digitalocean-pp-mcp

install-mcp:
	go install ./cmd/digitalocean-pp-mcp

build-all: build build-mcp
