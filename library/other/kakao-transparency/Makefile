.PHONY: build test lint install clean

build:
	go build -o bin/kakao-transparency-pp-cli ./cmd/kakao-transparency-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/kakao-transparency-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/kakao-transparency-pp-mcp ./cmd/kakao-transparency-pp-mcp

install-mcp:
	go install ./cmd/kakao-transparency-pp-mcp

build-all: build build-mcp
