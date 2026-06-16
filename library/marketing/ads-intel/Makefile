.PHONY: fmt test build doctor smoke

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

build:
	go build ./cmd/ads-intel-pp-cli

doctor:
	go run ./cmd/ads-intel-pp-cli --agent doctor

smoke:
	tmp=$$(mktemp -d); go run ./cmd/ads-intel-pp-cli --home "$$tmp" --profile smoke sync; go run ./cmd/ads-intel-pp-cli --home "$$tmp" --profile smoke --agent audit
