BINARY := insurance-finder
PKG := ./cmd/insurance-finder-pp-cli

.PHONY: all build test vet fmt clean run

all: vet test build

build:
	go build -o $(BINARY) $(PKG)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

clean:
	rm -f $(BINARY)

# Quick end-to-end demo against a throwaway profile.
run: build
	./$(BINARY) --help
