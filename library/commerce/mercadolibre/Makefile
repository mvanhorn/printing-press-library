.PHONY: build test lint install clean

build:
	go build -o bin/mercadolibre-pp-cli ./cmd/mercadolibre-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/mercadolibre-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/mercadolibre-pp-mcp ./cmd/mercadolibre-pp-mcp

install-mcp:
	go install ./cmd/mercadolibre-pp-mcp

build-all: build build-mcp
