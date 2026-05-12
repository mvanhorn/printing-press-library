.PHONY: build test lint install clean

build:
	go build -o bin/appsflyer-pp-cli ./cmd/appsflyer-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/appsflyer-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/appsflyer-pp-mcp ./cmd/appsflyer-pp-mcp

install-mcp:
	go install ./cmd/appsflyer-pp-mcp

build-all: build build-mcp
