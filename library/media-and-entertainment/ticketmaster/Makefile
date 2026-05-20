.PHONY: build test lint install clean

build:
	go build -o bin/ticketmaster-pp-cli ./cmd/ticketmaster-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ticketmaster-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ticketmaster-pp-mcp ./cmd/ticketmaster-pp-mcp

install-mcp:
	go install ./cmd/ticketmaster-pp-mcp

build-all: build build-mcp
