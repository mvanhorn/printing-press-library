BIN := jinko-pp-cli
PKG := ./cmd/$(BIN)
OUT := ./bin/$(BIN)
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo 0.1.0)
LDFLAGS := -ldflags "-X github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/cmd.Version=$(VERSION)"

.PHONY: build vet test lint fmt help install clean

build:
	go build $(LDFLAGS) -o $(OUT) $(PKG)

vet:
	go vet ./...

test:
	go test -v ./...

lint:
	golangci-lint run --timeout=5m

fmt:
	gofmt -w .
	goimports -w . 2>/dev/null || true

install:
	go install $(LDFLAGS) $(PKG)

help: build
	$(OUT) --help

clean:
	rm -rf bin/
