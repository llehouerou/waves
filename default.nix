{ pkgs, version }:
pkgs.buildGoModule {
  pname = "waves";
  inherit version;

  src = ./.;

  doCheck = true;

  vendorHash = "sha256-jUwA7KOHceuwSUMDzex8MHCBYvMPRkfkyk5THvXsF+M=";

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
}
