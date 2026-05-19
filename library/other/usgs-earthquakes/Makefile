.PHONY: build test lint install clean

build:
	go build -o bin/usgs-earthquakes-pp-cli ./cmd/usgs-earthquakes-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/usgs-earthquakes-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/usgs-earthquakes-pp-mcp ./cmd/usgs-earthquakes-pp-mcp

install-mcp:
	go install ./cmd/usgs-earthquakes-pp-mcp

build-all: build build-mcp
