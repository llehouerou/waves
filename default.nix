{ pkgs, version }:
pkgs.buildGoModule {
  pname = "waves";
  inherit version;

  src = ./.;

  doCheck = true;

  vendorHash = "sha256-1me2k4O9lc767i2cKH5FkbwzWDr3yOdBaNG1fqa4i20=";

  buildInputs = with pkgs; [
    alsa-lib
  ];

  nativeBuildInputs = with pkgs; [
    pkg-config
  ];

  ldflags = [
    "-s"
    "-w"
    "-X github.com/llehouerou/waves/internal/version.Version=${version}"
  ];

  meta = with pkgs.lib; {
    description = "Keyboard-driven terminal music player with Soulseek downloads and Last.fm integration";
    homepage = "https://github.com/llehouerou/waves";
    license = licenses.gpl3;
    maintainers = [ ];
    mainProgram = "waves";
  };
}
