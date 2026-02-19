{ pkgs, version }:
pkgs.buildGoModule {
  pname = "waves";
  inherit version;

  src = ./.;

  doCheck = true;

  vendorHash = "sha256-mUifSJ8IalfzqQHeDpFp0jbtZDr7OWPq16st9RJVu7U=";

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
