.PHONY: build test lint install clean

build:
	go build -o bin/googlehealth-pp-cli ./cmd/googlehealth-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/googlehealth-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/googlehealth-pp-mcp ./cmd/googlehealth-pp-mcp

install-mcp:
	go install ./cmd/googlehealth-pp-mcp

build-all: build build-mcp
