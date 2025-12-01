.PHONY: tools fmt lint test check build run install-hooks

# Install/update tools
tools:
	go install github.com/incu6us/goimports-reviser/v3@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# Format all Go files
fmt: tools
	goimports-reviser -format -recursive .

# Lint
lint: tools
	golangci-lint run

# Run tests (use PKG=./path/to/package to test specific package)
test:
ifdef PKG
	go test -v $(PKG)
else
	go test ./...
endif

# Format, lint, and test
check: fmt lint test

# Build (verify compilation)
build:
	go build -o /dev/null .

# Run the app
run:
	go run .

# Install git hooks
install-hooks:
	cp scripts/pre-commit .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit
