.PHONY: build test lint install clean

build:
	go build -o bin/booking-com-pp-cli ./cmd/booking-com-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/booking-com-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/booking-com-pp-mcp ./cmd/booking-com-pp-mcp

install-mcp:
	go install ./cmd/booking-com-pp-mcp

build-all: build build-mcp
