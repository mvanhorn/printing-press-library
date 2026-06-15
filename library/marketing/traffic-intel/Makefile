BINARY := traffic-intel-pp-cli
CMD := ./cmd/traffic-intel-pp-cli
BUILD_DIR ?= bin
HOME_DIR ?= $(CURDIR)/.tmp/traffic-intel-home

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
	TRAFFIC_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --agent agent-context
	TRAFFIC_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --agent doctor
	TRAFFIC_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --profile demo sync
	TRAFFIC_INTEL_HOME=$(HOME_DIR) $(BUILD_DIR)/$(BINARY) --profile demo digest weekly

clean:
	rm -rf $(BUILD_DIR) .tmp
