.PHONY: build test lint install clean

build:
	go build -o bin/foxnews-pp-cli ./cmd/foxnews-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/foxnews-pp-cli

clean:
	rm -rf bin/
