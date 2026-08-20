.PHONY: build test lint install clean

build:
	go build -o bin/myfitnesspal-pp-cli ./cmd/myfitnesspal-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/myfitnesspal-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/myfitnesspal-pp-mcp ./cmd/myfitnesspal-pp-mcp

install-mcp:
	go install ./cmd/myfitnesspal-pp-mcp

build-all: build build-mcp
