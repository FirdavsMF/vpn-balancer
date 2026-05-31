.PHONY: build test lint clean install run fmt vet deps

BINARY_NAME=vpn-balancer
BUILD_DIR=./bin
VERSION=0.1.0
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

build:
@echo "Building $(BINARY_NAME) v$(VERSION)..."
@mkdir -p $(BUILD_DIR)
go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/vpn-balancer
@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

test:
@echo "Running tests..."
go test -v ./...

lint:
@echo "Running linter..."
@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run ./...

fmt:
@echo "Formatting code..."
go fmt ./...

vet:
@echo "Running go vet..."
go vet ./...

clean:
@echo "Cleaning..."
rm -rf $(BUILD_DIR)
go clean

install: build
@echo "Installing..."
cp $(BUILD_DIR)/$(BINARY_NAME) $$(go env GOPATH)/bin/$(BINARY_NAME)

run: build
./$(BUILD_DIR)/$(BINARY_NAME)

deps:
@echo "Updating dependencies..."
go mod tidy
go mod verify

check: fmt vet test
@echo "All checks passed!"
