.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/pocket-concierge-pp-cli$(BIN_EXT) ./cmd/pocket-concierge-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pocket-concierge-pp-cli

clean:
	rm -rf bin/

