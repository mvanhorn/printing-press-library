.PHONY: build test lint install clean

build:
	go build -o bin/agentpool-pp-cli ./cmd/agentpool-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/agentpool-pp-cli

clean:
	rm -rf bin/
