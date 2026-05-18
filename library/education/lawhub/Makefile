APP := lawhub-pp-cli
BIN := bin/$(APP)-go
GO ?= go
VERSION ?= 0.1.0-dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/mvanhorn/printing-press-library/library/education/lawhub/internal/cli.version=$(VERSION) -X github.com/mvanhorn/printing-press-library/library/education/lawhub/internal/cli.commit=$(COMMIT) -X github.com/mvanhorn/printing-press-library/library/education/lawhub/internal/cli.date=$(DATE)

.PHONY: build test vet smoke clean

build:
	mkdir -p bin
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/lawhub-pp-cli

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

smoke: build
	./$(BIN) --help >/dev/null
	./$(BIN) version --agent >/dev/null
	./$(BIN) doctor --agent >/dev/null

clean:
	rm -rf bin
