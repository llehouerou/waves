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
        packages.default = pkgs.buildGoModule {
          pname = "waves";
          inherit version;

          src = ./.;

          vendorHash = "sha256-IzUmzAy8ODg0I92pTk2hrS4LZ/JoJrFcARvFP5fXhqY=";

          buildInputs = with pkgs; [
            alsa-lib
          ];

          nativeBuildInputs = with pkgs; [
            pkg-config
          ];

          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];

          meta = with pkgs.lib; {
            description = "Terminal-based music player";
            homepage = "https://github.com/llehouerou/waves";
            license = licenses.gpl3;
            maintainers = [ ];
            mainProgram = "waves";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go toolchain
            go
            gopls
            golines
            goimports-reviser
            golangci-lint
            delve

            # Nix tooling
            nil

            # Build dependencies
            alsa-lib
            pkg-config
            gnumake
          ];

          shellHook = ''
            export GOPATH="$HOME/go"
            export PATH="$GOPATH/bin:$PATH"
          '';
        };
      }
    );
}
