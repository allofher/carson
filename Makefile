.PHONY: build install test clean generate

# The embedded default is the single source of truth.
# This copies it to the repo root so humans can reference it.
generate:
	cp internal/config/config.default.json config.example.json

# Build the carson binary in the project root.
build: generate
	go build -o carson ./cmd/carson/

# Install to $GOPATH/bin (or $HOME/go/bin by default).
install: generate
	go install ./cmd/carson/

# Run all tests.
test:
	go test ./...

# Remove build artifacts.
clean:
	rm -f carson
