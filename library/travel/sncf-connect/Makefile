.PHONY: build test lint install clean

build:
	go build -o bin/sncf-connect-pp-cli ./cmd/sncf-connect-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sncf-connect-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sncf-connect-pp-mcp ./cmd/sncf-connect-pp-mcp

install-mcp:
	go install ./cmd/sncf-connect-pp-mcp

build-all: build build-mcp
