.PHONY: build test clean install release-local

# Default target
all: build

# Build the binary
build:
	cd cmd/imgup && go build -ldflags "-X 'main.version=$$(git describe --tags --always --dirty)' -X 'main.commit=$$(git rev-parse --short HEAD)' -X 'main.date=$$(date -u +%Y-%m-%dT%H:%M:%SZ)'" -o ../../imgup .

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f imgup
	rm -rf dist/

# Install locally
install: build
	sudo cp imgup /usr/local/bin/

# Build for all platforms locally (without releasing)
release-local:
	goreleaser release --snapshot --clean

# Run goreleaser in release mode (requires GITHUB_TOKEN)
release:
	goreleaser release --clean

# Quick development test
dev-test: build
	./imgup upload tests/fixtures/test_metadata.jpeg

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Update dependencies
deps:
	go mod tidy
	go mod verify
