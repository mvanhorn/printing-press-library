.PHONY: build test lint install clean

build:
	go build -o bin/bloomest-laundry-manager-pp-cli ./cmd/bloomest-laundry-manager-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bloomest-laundry-manager-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bloomest-laundry-manager-pp-mcp ./cmd/bloomest-laundry-manager-pp-mcp

install-mcp:
	go install ./cmd/bloomest-laundry-manager-pp-mcp

build-all: build build-mcp
