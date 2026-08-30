.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/orcarouter-pp-cli$(BIN_EXT) ./cmd/orcarouter-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/orcarouter-pp-cli

clean:
	rm -rf bin/
