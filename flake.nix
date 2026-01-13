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

          vendorHash = "sha256-z02x4MgLjiUYm8FBl8W8hsy8IIJAGmjLozc70OktCSw=";

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
            go
            gopls
            golangci-lint
            alsa-lib
            pkg-config
          ];
        };
      }
    );
}
