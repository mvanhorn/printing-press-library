.PHONY: build test lint install clean

build:
	go build -o bin/airbnb-vrbo-pp-cli ./cmd/airbnb-vrbo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/airbnb-vrbo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/airbnb-vrbo-pp-mcp ./cmd/airbnb-vrbo-pp-mcp

install-mcp:
	go install ./cmd/airbnb-vrbo-pp-mcp

build-all: build build-mcp
