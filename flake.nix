{
  description = "Terminal-based music player";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = if (self ? shortRev) then self.shortRev else "dev";
      in
      {
        packages.default = import ./default.nix { inherit pkgs version; };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go toolchain
            go
            gopls
            golines
            goimports-reviser
            delve

            # Nix tooling
            nil

            # Build dependencies
            alsa-lib
            pkg-config
            gnumake

            # Test dependencies
            ffmpeg

            # MCP server dependencies
            pipx
            nodejs
          ];

          shellHook = ''
            export GOPATH="$HOME/go"
            export PATH="$GOPATH/bin:$PATH"

            GOLANGCI_LINT_VERSION=$(cat .golangci-lint-version 2>/dev/null || echo "latest")
            if ! golangci-lint --version 2>/dev/null | grep -q "''${GOLANGCI_LINT_VERSION#v}"; then
              echo "Installing golangci-lint ''${GOLANGCI_LINT_VERSION}..."
              go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@''${GOLANGCI_LINT_VERSION}"
            fi
          '';
        };
      }
    );
}
