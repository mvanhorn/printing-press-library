BINARY := apple-voice-memos-pp-cli
VERSION ?= dev

.PHONY: build test check install clean

build:
	mkdir -p bin
	go build -trimpath -ldflags "-X github.com/mvanhorn/printing-press-library/library/productivity/apple-voice-memos/internal/cli.version=$(VERSION)" -o bin/$(BINARY) ./cmd/$(BINARY)

test:
	go test -race ./...

check:
	test -z "$$(gofmt -l .)"
	go test -race ./...
	go vet ./...
	govulncheck ./...
	@tmpdir=$$(mktemp -d); trap 'rm -rf "$$tmpdir"' EXIT; go build -o "$$tmpdir/$(BINARY)" ./cmd/$(BINARY)

install: build
	mkdir -p $(HOME)/.local/bin
	cp bin/$(BINARY) $(HOME)/.local/bin/$(BINARY)

clean:
	rm -rf bin dist
