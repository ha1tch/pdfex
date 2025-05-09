# Makefile for pdfex project

# Go variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOCLEAN := $(GOCMD) clean
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt -s -w
GOVET := $(GOCMD) vet

# Binary names
BINARY_NAME := pdfex
EXAMPLE_BINARY := basic_extraction

# Source directories
CMD_DIR := ./cmd/pdfex
EXAMPLE_DIR := ./examples/basic_extraction
SRC_DIRS := ./internal/... ./pkg/...

# Output directories
BIN_DIR := ./bin
DIST_DIR := ./dist

# Version info
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0-dev")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X github.com/yourusername/pdfex/pkg/pdfex.version=$(VERSION) -X github.com/yourusername/pdfex/pkg/pdfex.buildDate=$(BUILD_DATE) -X github.com/yourusername/pdfex/pkg/pdfex.commitHash=$(COMMIT_HASH)"

# Operating systems and architectures for cross-compilation
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build clean test fmt lint vet coverage example deps tidy cross-compile install help

# Default target - build the CLI tool
all: clean tidy fmt vet build

# Build the CLI tool
build:
	@mkdir -p $(BIN_DIR)
	@echo "Building pdfex CLI tool..."
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "✓ Build successful: $(BIN_DIR)/$(BINARY_NAME)"

# Build the example application
example:
	@mkdir -p $(BIN_DIR)
	@echo "Building example application..."
	$(GOBUILD) -o $(BIN_DIR)/$(EXAMPLE_BINARY) $(EXAMPLE_DIR)
	@echo "✓ Build successful: $(BIN_DIR)/$(EXAMPLE_BINARY)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BIN_DIR) $(DIST_DIR)
	@echo "✓ Cleaned up build artifacts"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v $(SRC_DIRS)

# Generate test coverage report
coverage:
	@echo "Generating coverage report..."
	$(GOTEST) -coverprofile=coverage.out $(SRC_DIRS)
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) .
	@echo "✓ Code formatted"

# Lint code
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint is not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi
	@echo "✓ Code linted"

# Vet code
vet:
	@echo "Vetting code..."
	$(GOVET) ./...
	@echo "✓ Code vetted"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOGET) -v -t -d ./...
	@echo "✓ Dependencies downloaded"

# Tidy up module files
tidy:
	@echo "Tidying Go modules..."
	$(GOMOD) tidy
	@echo "✓ Go modules tidied"

# Cross-compile for different platforms
cross-compile:
	@echo "Cross-compiling for multiple platforms..."
	@mkdir -p $(DIST_DIR)
	$(foreach platform,$(PLATFORMS),\
		$(eval OS_ARCH := $(subst /, ,$platform))\
		$(eval OS := $(word 1,$(OS_ARCH)))\
		$(eval ARCH := $(word 2,$(OS_ARCH)))\
		$(eval SUFFIX := $(if $(findstring windows,$(OS)),.exe,))\
		echo "Building for $(OS)/$(ARCH)..." && \
		GOOS=$(OS) GOARCH=$(ARCH) $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-$(OS)-$(ARCH)$(SUFFIX) $(CMD_DIR) && \
		echo "✓ Built $(DIST_DIR)/$(BINARY_NAME)-$(OS)-$(ARCH)$(SUFFIX)"; \
	)
	@echo "✓ Cross-compilation complete"

# Create distribution archives
dist: cross-compile
	@echo "Creating distribution archives..."
	@mkdir -p $(DIST_DIR)/archives
	$(foreach platform,$(PLATFORMS),\
		$(eval OS_ARCH := $(subst /, ,$platform))\
		$(eval OS := $(word 1,$(OS_ARCH)))\
		$(eval ARCH := $(word 2,$(OS_ARCH)))\
		$(eval SUFFIX := $(if $(findstring windows,$(OS)),.exe,))\
		$(eval ARCHIVE_NAME := $(BINARY_NAME)-$(VERSION)-$(OS)-$(ARCH))\
		$(eval ARCHIVE_FORMAT := $(if $(findstring windows,$(OS)),zip,tar.gz))\
		echo "Creating archive for $(OS)/$(ARCH)..." && \
		mkdir -p $(DIST_DIR)/tmp/$(ARCHIVE_NAME) && \
		cp $(DIST_DIR)/$(BINARY_NAME)-$(OS)-$(ARCH)$(SUFFIX) $(DIST_DIR)/tmp/$(ARCHIVE_NAME)/$(BINARY_NAME)$(SUFFIX) && \
		cp README.md LICENSE $(DIST_DIR)/tmp/$(ARCHIVE_NAME)/ && \
		$(if $(findstring zip,$(ARCHIVE_FORMAT)),\
			cd $(DIST_DIR)/tmp && zip -r ../archives/$(ARCHIVE_NAME).zip $(ARCHIVE_NAME),\
			tar -czf $(DIST_DIR)/archives/$(ARCHIVE_NAME).tar.gz -C $(DIST_DIR)/tmp $(ARCHIVE_NAME)\
		) && \
		rm -rf $(DIST_DIR)/tmp/$(ARCHIVE_NAME) && \
		echo "✓ Created $(DIST_DIR)/archives/$(ARCHIVE_NAME).$(ARCHIVE_FORMAT)"; \
	)
	rm -rf $(DIST_DIR)/tmp
	@echo "✓ Distribution archives created"

# Install locally
install:
	@echo "Installing pdfex CLI tool..."
	$(GOCMD) install $(LDFLAGS) $(CMD_DIR)
	@echo "✓ Installed pdfex CLI tool"

# Run CLI tool with sample arguments (for development)
run:
	@if [ -x "$(BIN_DIR)/$(BINARY_NAME)" ]; then \
		echo "Running pdfex..."; \
		$(BIN_DIR)/$(BINARY_NAME) $(ARGS); \
	else \
		echo "pdfex binary not found. Run 'make build' first."; \
		exit 1; \
	fi

# Create directories for project structure
init:
	@echo "Creating project directory structure..."
	@mkdir -p cmd/pdfex
	@mkdir -p internal/document
	@mkdir -p internal/content
	@mkdir -p internal/text
	@mkdir -p internal/metrics
	@mkdir -p internal/utils
	@mkdir -p pkg/pdfex
	@mkdir -p examples/basic_extraction
	@echo "✓ Project structure created"

# Show help for available targets
help:
	@echo "pdfex - PDF Extraction Library and Tool"
	@echo "Available targets:"
	@echo "  make              : Build the CLI tool (same as 'make build')"
	@echo "  make build        : Build the CLI tool"
	@echo "  make example      : Build the example application"
	@echo "  make clean        : Clean build artifacts"
	@echo "  make test         : Run tests"
	@echo "  make coverage     : Generate test coverage report"
	@echo "  make fmt          : Format code"
	@echo "  make lint         : Lint code"
	@echo "  make vet          : Vet code"
	@echo "  make deps         : Download dependencies"
	@echo "  make tidy         : Tidy up module files"
	@echo "  make cross-compile: Build for multiple platforms"
	@echo "  make dist         : Create distribution archives"
	@echo "  make install      : Install the CLI tool locally"
	@echo "  make run ARGS='..': Run the CLI tool with specified arguments"
	@echo "  make init         : Create project directory structure"
	@echo "  make help         : Show this help"
	@echo
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT_HASH)"
	@echo "Build Date: $(BUILD_DATE)"