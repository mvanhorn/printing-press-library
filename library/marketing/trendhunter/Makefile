.PHONY: build test lint install clean

build:
	go build -o bin/trendhunter-pp-cli ./cmd/trendhunter-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/trendhunter-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/trendhunter-pp-mcp ./cmd/trendhunter-pp-mcp

install-mcp:
	go install ./cmd/trendhunter-pp-mcp

build-all: build build-mcp
