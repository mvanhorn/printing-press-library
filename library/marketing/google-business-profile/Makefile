.PHONY: build test lint install clean

build:
	go build -o bin/google-business-profile-pp-cli ./cmd/google-business-profile-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/google-business-profile-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/google-business-profile-pp-mcp ./cmd/google-business-profile-pp-mcp

install-mcp:
	go install ./cmd/google-business-profile-pp-mcp

build-all: build build-mcp
