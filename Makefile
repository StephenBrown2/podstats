# Makefile for Podcast Statistics Analyzer
.PHONY: build clean test run-sample lint format
# set unstable := true

# Default target
build: generate
	go build -o podstats .

# Clean build artifacts
clean:
	rm -f podstats

# Run tests (if any test files exist)
test:
	go test ./...

# Run with sample OPML file
run-sample: build
	./podstats sample.opml

# Install dependencies (if needed)
deps:
	go mod tidy

# Format code
fmt:
	go fmt ./...

generate:
	go generate ./...

# Run linter
lint:
	golangci-lint run --fix

# Format code using golangci-lint v2 formatters (e.g., gofumpt)
format:
	golangci-lint fmt

# Build for different platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o podstats-linux-amd64 main.go
	GOOS=darwin GOARCH=amd64 go build -o podstats-darwin-amd64 main.go
	GOOS=windows GOARCH=amd64 go build -o podstats-windows-amd64.exe main.go
