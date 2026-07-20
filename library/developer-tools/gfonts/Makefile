.PHONY: build install clean test version

VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
BINARY := gfonts-pp-cli
PREFIX ?= $(HOME)/.local/bin

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/gfonts-pp-cli

install: build
	mkdir -p $(PREFIX)
	cp $(BINARY) $(PREFIX)/$(BINARY)
	chmod +x $(PREFIX)/$(BINARY)
	@echo "Installed $(BINARY) to $(PREFIX)/$(BINARY)"

clean:
	rm -f $(BINARY)

test: build
	./$(BINARY) version
	./$(BINARY) categories
	./$(BINARY) search serif
	@echo "All smoke tests passed."

uninstall:
	rm -f $(PREFIX)/$(BINARY)
	@echo "Removed $(PREFIX)/$(BINARY)"
