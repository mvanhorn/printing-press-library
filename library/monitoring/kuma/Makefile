.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/kuma-pp-cli$(BIN_EXT) ./cmd/kuma-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/kuma-pp-cli

clean:
	rm -rf bin/

build-all: build
