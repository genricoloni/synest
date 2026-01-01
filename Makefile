.PHONY: run build test clean lint

# Binary name
BINARY_NAME=synest
BIN_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOMOD=$(GOCMD) mod

# Main package path
MAIN_PATH=./cmd/daemon

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BIN_DIR)/$(BINARY_NAME)"

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	$(GORUN) $(MAIN_PATH)

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) ./... -v

# Run tests with coverage
COVERAGE_FILE := coverage.out
COVERAGE_CLEAN := coverage.clean.out
COVERAGE_HTML := coverage.html

test-coverage:
	@echo "==> Cleaning previous coverage profiles..."
	@rm -f $(COVERAGE_FILE) $(COVERAGE_CLEAN) $(COVERAGE_HTML)

	@echo "==> Running tests with covermode=atomic..."
	@# -covermode=atomic is essential for concurrent code (goroutines)
	go test ./... -v -covermode=atomic -coverprofile=$(COVERAGE_FILE)
	
	@echo "==> Filtering generated files and boilerplate..."
	@# Exclude mocks, generated files and dbus_client adapter for accurate percentage
	@grep -vE "_mock.go|dbus_client.go|main.go" $(COVERAGE_FILE) > $(COVERAGE_CLEAN)
	
	@echo "==> Coverage summary by function:"
	@go tool cover -func=$(COVERAGE_CLEAN)
	
	@echo "==> Generating HTML report..."
	@go tool cover -html=$(COVERAGE_CLEAN) -o $(COVERAGE_HTML)
	@echo "Report generated: $(COVERAGE_HTML)"

	@xdg-open $(COVERAGE_HTML) || open $(COVERAGE_HTML) || echo "Please open $(COVERAGE_HTML) manually."

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html
	@echo "Clean complete"

# Run linter (requires golangci-lint to be installed)
lint:
	@echo "Running golangci-lint..."
	golangci-lint run ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

# Generate mocks for testing
generate:
	@echo "Generating mocks..."
	$(GOCMD) generate ./...
	@echo "Mock generation complete"

# Install the binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	cp $(BIN_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete"

# Help
help:
	@echo "Synest Makefile Commands:"
	@echo "  make build         - Build the binary"
	@echo "  make run           - Run the application"
	@echo "  make test          - Run all tests"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make lint          - Run golangci-lint"
	@echo "  make tidy          - Tidy go.mod"
	@echo "  make deps          - Download dependencies"
	@echo "  make generate      - Generate mocks for testing"
	@echo "  make install       - Install binary to /usr/local/bin"
	@echo "  make deps          - Download dependencies"
	@echo "  make install       - Install binary to /usr/local/bin"
