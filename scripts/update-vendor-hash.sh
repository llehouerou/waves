#!/usr/bin/env bash
set -euo pipefail

# Updates the vendorHash in default.nix by computing it from go.mod/go.sum

NIX_FILE="default.nix"

if [[ ! -f "$NIX_FILE" ]]; then
    echo "Error: $NIX_FILE not found" >&2
    exit 1
fi

# Save current hash in case we need to restore
CURRENT_HASH=$(grep -oP 'vendorHash = "\Ksha256-[^"]+' "$NIX_FILE" || echo "")

echo "Setting vendorHash to empty string..."
sed -i 's|vendorHash = "sha256-[^"]*";|vendorHash = "";|' "$NIX_FILE"

echo "Running nix build to compute hash..."
# Capture the error output which contains the expected hash
BUILD_OUTPUT=$(nix build 2>&1 || true)

# Extract the hash from the error message
# The format is: specified: sha256-AAAA...  got: sha256-XXXX...
EXPECTED_HASH=$(echo "$BUILD_OUTPUT" | grep -oP 'got:\s+\Ksha256-[A-Za-z0-9+/=]+' | head -1)

if [[ -z "$EXPECTED_HASH" ]]; then
    echo "Error: Could not extract hash from nix build output" >&2
    echo "Build output:" >&2
    echo "$BUILD_OUTPUT" >&2
    # Restore previous hash
    if [[ -n "$CURRENT_HASH" ]]; then
        sed -i "s|vendorHash = \"\";|vendorHash = \"$CURRENT_HASH\";|" "$NIX_FILE"
    else
        sed -i 's|vendorHash = "";|vendorHash = "sha256-FIXME";|' "$NIX_FILE"
    fi
    exit 1
fi

echo "Updating default.nix with hash: $EXPECTED_HASH"
sed -i "s|vendorHash = \"\";|vendorHash = \"$EXPECTED_HASH\";|" "$NIX_FILE"

echo "Verifying build..."
if nix build; then
    echo "Success! vendorHash updated."
else
    echo "Error: Build still failed after updating hash" >&2
    exit 1
fi
