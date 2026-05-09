.PHONY: build test install clean

build:
	go build -o bin/obs-pp-cli ./cmd/obs-pp-cli

test:
	go test ./...

install:
	go install ./cmd/obs-pp-cli

clean:
	rm -rf bin/
