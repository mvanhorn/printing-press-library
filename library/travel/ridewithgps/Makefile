.PHONY: build test lint install clean

build:
	go build -o bin/ridewithgps-pp-cli ./cmd/ridewithgps-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ridewithgps-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ridewithgps-pp-mcp ./cmd/ridewithgps-pp-mcp

install-mcp:
	go install ./cmd/ridewithgps-pp-mcp

build-all: build build-mcp
