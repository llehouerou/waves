{ pkgs, version }:
pkgs.buildGoModule {
  pname = "waves";
  inherit version;

  src = ./.;

  doCheck = true;

  # One go test over every package: buildGoModule's default runs them one after the other.
  checkPhase = ''
    runHook preCheck
    export GOFLAGS=''${GOFLAGS//-trimpath/}
    go test -p "$NIX_BUILD_CORES" -vet=off $checkFlags $(getGoDirs test)
    runHook postCheck
  '';

  vendorHash = "sha256-CpAkWIJZJokJZhGMtr2h/aXDRV1JIsf3nVhNV5U0Bg0=";

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
