.PHONY: build test lint install clean

build:
	go build -o bin/booking-com-demand-pp-cli ./cmd/booking-com-demand-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/booking-com-demand-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/booking-com-demand-pp-mcp ./cmd/booking-com-demand-pp-mcp

install-mcp:
	go install ./cmd/booking-com-demand-pp-mcp

build-all: build build-mcp
