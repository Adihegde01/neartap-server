BINARY=neartap-server
BUILD_DIR=./bin
MAIN=./cmd/server/main.go

.PHONY: build run dev clean test lint

## build: compile the server binary
build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) $(MAIN)
	@echo "✅ Built $(BUILD_DIR)/$(BINARY)"

## run: build and run
run: build
	$(BUILD_DIR)/$(BINARY)

## dev: run with hot reload using air (install with: go install github.com/air-verse/air@latest)
dev:
	@which air > /dev/null || (echo "❌ air not found. Install with: go install github.com/air-verse/air@latest" && exit 1)
	air -c .air.toml

## test: run all tests
test:
	go test ./... -v -race -timeout 30s

## lint: run golangci-lint
lint:
	@which golangci-lint > /dev/null || (echo "❌ golangci-lint not found" && exit 1)
	golangci-lint run ./...

## tidy: tidy go modules
tidy:
	go mod tidy

## clean: remove build artifacts
clean:
	rm -rf $(BUILD_DIR)

## deps: download dependencies
deps:
	go mod download

help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
