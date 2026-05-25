.PHONY: build test lint install clean

build:
	go build -o bin/epropertyplus-pp-cli ./cmd/epropertyplus-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/epropertyplus-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/epropertyplus-pp-mcp ./cmd/epropertyplus-pp-mcp

install-mcp:
	go install ./cmd/epropertyplus-pp-mcp

build-all: build build-mcp
