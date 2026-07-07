.PHONY: build test lint install clean

build:
	go build -o bin/thelancet-pp-cli ./cmd/thelancet-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/thelancet-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/thelancet-pp-mcp ./cmd/thelancet-pp-mcp

install-mcp:
	go install ./cmd/thelancet-pp-mcp

build-all: build build-mcp
