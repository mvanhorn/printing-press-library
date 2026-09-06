.PHONY: build build-mcp build-all test lint install install-mcp clean

build:
	go build -o bin/homeclaw-pp-cli ./cmd/homeclaw-pp-cli

build-mcp:
	go build -o bin/homeclaw-pp-mcp ./cmd/homeclaw-pp-mcp

build-all: build build-mcp

test:
	go test ./...

lint:
	gofmt -d .

install:
	go install ./cmd/homeclaw-pp-cli

install-mcp:
	go install ./cmd/homeclaw-pp-mcp

clean:
	rm -rf bin/
