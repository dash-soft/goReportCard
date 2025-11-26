.PHONY: build clean fmt vet all

# Build the application
build:
	go build -o bin/report ./cmd/app

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Check formatting
check-fmt:
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "The following files are not properly formatted:"; \
		gofmt -s -l .; \
		exit 1; \
	fi

# Run all checks
check: check-fmt vet

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f *.pdf

# Install dependencies
deps:
	go mod download
	go mod tidy

# Build for all platforms (for release preparation)
build-all:
	@echo "Building for Linux amd64..."
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/report-linux-amd64 ./cmd/app
	@echo "Building for Linux arm64..."
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/report-linux-arm64 ./cmd/app
	@echo "Building for macOS amd64..."
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/report-darwin-amd64 ./cmd/app
	@echo "Building for macOS arm64..."
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/report-darwin-arm64 ./cmd/app
	@echo "Building for Windows amd64..."
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/report-windows-amd64.exe ./cmd/app
	@echo "Building for Windows 386..."
	GOOS=windows GOARCH=386 go build -ldflags="-s -w" -o bin/report-windows-386.exe ./cmd/app

# Create example PDF from showcase.md
example:
	go run ./cmd/app showcase.md example.pdf

# Development setup
dev-setup: deps
	go install github.com/cosmtrek/air@latest

# Run with live reload (requires air)
dev:
	air

# All tasks
all: check build
