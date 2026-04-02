{ pkgs, version }:
pkgs.buildGoModule {
  pname = "waves";
  inherit version;

  src = ./.;

  doCheck = true;

  vendorHash = "sha256-lps0OdY8KoILJh/roY78iC+bYHPeENioQoIsL6v/N0A=";

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
    description = "Keyboard-driven terminal music player with Soulseek downloads and Last.fm integration";
    homepage = "https://github.com/llehouerou/waves";
    license = licenses.gpl3;
    maintainers = [ ];
    mainProgram = "waves";
  };
}
