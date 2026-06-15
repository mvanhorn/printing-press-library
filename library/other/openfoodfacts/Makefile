.PHONY: build test lint install clean

build:
	go build -o bin/openfoodfacts-pp-cli ./cmd/openfoodfacts-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/openfoodfacts-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/openfoodfacts-pp-mcp ./cmd/openfoodfacts-pp-mcp

install-mcp:
	go install ./cmd/openfoodfacts-pp-mcp

build-all: build build-mcp
