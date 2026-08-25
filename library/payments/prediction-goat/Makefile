.PHONY: build test lint install clean

build:
	go build -o bin/prediction-goat-pp-cli ./cmd/prediction-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/prediction-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/prediction-goat-pp-mcp ./cmd/prediction-goat-pp-mcp

install-mcp:
	go install ./cmd/prediction-goat-pp-mcp

build-all: build build-mcp
