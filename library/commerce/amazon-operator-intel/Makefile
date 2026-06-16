BINARY := amazon-operator-intel-pp-cli
CMD := ./cmd/amazon-operator-intel-pp-cli
BUILD_DIR ?= bin
HOME_DIR ?= $(CURDIR)/.tmp/amazon-operator-intel-home

.PHONY: all fmt vet test build doctor smoke clean

all: fmt vet test build

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

test:
	go test ./...

build:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

doctor:
	go run $(CMD) --agent doctor
	go run $(CMD) --agent sources doctor

smoke: build
	AMAZON_OPERATOR_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --agent agent-context
	AMAZON_OPERATOR_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --agent doctor
	AMAZON_OPERATOR_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --profile demo sync
	AMAZON_OPERATOR_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --profile demo --agent war-room
	AMAZON_OPERATOR_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --profile demo --agent operator-plan

clean:
	rm -rf $(BUILD_DIR) .tmp
