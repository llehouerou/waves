# PROTOTYPE (throwaway): build/run environment for the Gio feel check.
# Run: nix-shell prototypes/gio-feel --run 'cd prototypes/gio-feel && go run -tags nox11 .'
{ pkgs ? import <nixpkgs> { } }:
pkgs.mkShell {
  nativeBuildInputs = [ pkgs.go pkgs.pkg-config ];
  buildInputs = with pkgs; [ wayland libxkbcommon libGL vulkan-headers ];
  # Gio dlopens the Vulkan loader and EGL/GLES at runtime.
  LD_LIBRARY_PATH = pkgs.lib.makeLibraryPath (with pkgs; [ vulkan-loader libGL wayland libxkbcommon ]);
  PROTO_FONT_DIR = "${pkgs.ibm-plex}/share/fonts/truetype";
  # The repo's untracked go.work does not list this throwaway module.
  GOWORK = "off";
}
