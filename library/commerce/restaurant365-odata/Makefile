.PHONY: build test lint install clean

build:
	go build -o bin/restaurant365-odata-pp-cli ./cmd/restaurant365-odata-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/restaurant365-odata-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/restaurant365-odata-pp-mcp ./cmd/restaurant365-odata-pp-mcp

install-mcp:
	go install ./cmd/restaurant365-odata-pp-mcp

build-all: build build-mcp
