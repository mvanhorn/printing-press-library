.PHONY: build test lint install clean

build:
	go build -o bin/pvgis-pp-cli ./cmd/pvgis-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pvgis-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pvgis-pp-mcp ./cmd/pvgis-pp-mcp

install-mcp:
	go install ./cmd/pvgis-pp-mcp

build-all: build build-mcp
