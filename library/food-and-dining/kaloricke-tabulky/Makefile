.PHONY: build test lint install clean

build:
	go build -o bin/kaloricke-tabulky-pp-cli ./cmd/kaloricke-tabulky-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/kaloricke-tabulky-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/kaloricke-tabulky-pp-mcp ./cmd/kaloricke-tabulky-pp-mcp

install-mcp:
	go install ./cmd/kaloricke-tabulky-pp-mcp

build-all: build build-mcp
