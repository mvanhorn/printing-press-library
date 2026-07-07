.PHONY: build test lint install clean

build:
	go build -o bin/sutra-fitness-pp-cli ./cmd/sutra-fitness-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sutra-fitness-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sutra-fitness-pp-mcp ./cmd/sutra-fitness-pp-mcp

install-mcp:
	go install ./cmd/sutra-fitness-pp-mcp

build-all: build build-mcp
