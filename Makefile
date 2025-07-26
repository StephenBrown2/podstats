# Makefile for Podcast Statistics Analyzer

.PHONY: build clean test run-sample

# Default target
build:
	go build -o podstats main.go

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

# Run linter
lint:
	golangci-lint run

# Build for different platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o podstats-linux-amd64 main.go
	GOOS=darwin GOARCH=amd64 go build -o podstats-darwin-amd64 main.go
	GOOS=windows GOARCH=amd64 go build -o podstats-windows-amd64.exe main.go
