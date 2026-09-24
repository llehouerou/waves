.PHONY: fmt fmt-check lint test coverage check build run install-hooks update-vendor-hash clean

# Format all Go files (tools provided by nix devShell)
fmt:
	goimports-reviser -format -recursive .

# Fail on unformatted files without rewriting them (run `make fmt` to fix)
fmt-check:
	goimports-reviser -format -recursive -list-diff -set-exit-status .

# Lint
lint:
	golangci-lint run

# Run tests (use PKG=./path/to/package to test specific package)
test:
ifdef PKG
	go test -v $(PKG)
else
	go test ./...
endif

# Run tests with coverage (use PKG=./path/to/package for specific package)
coverage:
ifdef PKG
	go test -cover -coverprofile=coverage.out $(PKG)
else
	go test -cover -coverprofile=coverage.out ./...
endif
	go tool cover -func=coverage.out

# Format, lint, and test
check: fmt-check lint test

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

# Update vendorHash in flake.nix
update-vendor-hash:
	./scripts/update-vendor-hash.sh

# Clean Go build cache (run after nix flake update to avoid stale CGO bindings)
clean:
	go clean -cache
