.PHONY: build test lint install clean

build:
	go build -o bin/marketo-engage-pp-cli ./cmd/marketo-engage-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/marketo-engage-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/marketo-engage-pp-mcp ./cmd/marketo-engage-pp-mcp

install-mcp:
	go install ./cmd/marketo-engage-pp-mcp

build-all: build build-mcp
