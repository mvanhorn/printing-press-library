.PHONY: build test lint install clean

build:
	go build -o bin/american-express-offers-pp-cli ./cmd/american-express-offers-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/american-express-offers-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/american-express-offers-pp-mcp ./cmd/american-express-offers-pp-mcp

install-mcp:
	go install ./cmd/american-express-offers-pp-mcp

build-all: build build-mcp
