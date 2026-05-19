.PHONY: build test lint install clean

build:
	go build -o bin/naver-blog-pp-cli ./cmd/naver-blog-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/naver-blog-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/naver-blog-pp-mcp ./cmd/naver-blog-pp-mcp

install-mcp:
	go install ./cmd/naver-blog-pp-mcp

build-all: build build-mcp
