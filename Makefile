.PHONY: tools fmt lint build run commit push

# Install/update tools
tools:
	go install github.com/incu6us/goimports-reviser/v3@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Format all Go files
fmt: tools
	goimports-reviser -format -recursive .

# Lint
lint: tools
	golangci-lint run

# Build (verify compilation)
build:
	go build -o /dev/null .

# Run the app
run:
	go run .

# Commit with formatting and linting
# Usage: make commit m="message" [b="body"]
commit: fmt lint
	git add -A
	git commit -m "$(m)" $(if $(b),-m "$(b)",)

# Push to remote
push:
	git push -u origin main
