.PHONY: build test lint install clean

build:
	go build -o bin/servicetitan-pricebook-pp-cli ./cmd/servicetitan-pricebook-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/servicetitan-pricebook-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/servicetitan-pricebook-pp-mcp ./cmd/servicetitan-pricebook-pp-mcp

install-mcp:
	go install ./cmd/servicetitan-pricebook-pp-mcp

build-all: build build-mcp
