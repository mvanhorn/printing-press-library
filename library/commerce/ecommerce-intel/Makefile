BINARY := ecommerce-intel-pp-cli
CMD := ./cmd/ecommerce-intel-pp-cli
BUILD_DIR ?= bin
HOME_DIR ?= $(CURDIR)/.tmp/ecommerce-intel-home

.PHONY: all fmt test build smoke doctor clean

all: fmt test build

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

build:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

doctor:
	go run $(CMD) --agent doctor
	go run $(CMD) --agent sources doctor

smoke: build
	ECOMMERCE_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --agent agent-context
	ECOMMERCE_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --agent doctor
	ECOMMERCE_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --profile demo sync
	ECOMMERCE_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --profile demo digest weekly
	ECOMMERCE_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --profile demo geo-audit

clean:
	rm -rf $(BUILD_DIR) .tmp
