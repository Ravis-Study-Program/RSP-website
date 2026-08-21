{
  description = "Development environment for rsp-website";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = {nixpkgs, ...}: let
    systems = ["aarch64-darwin" "aarch64-linux" "x86_64-linux"];
    forEachSystem = nixpkgs.lib.genAttrs systems;
  in {
    devShells = forEachSystem (system: {
      default = let
        pkgs = nixpkgs.legacyPackages.${system};
        playwrightBrowsers = pkgs.playwright-driver.selectBrowsers {
          withFfmpeg = false;
        };
      in
        pkgs.mkShell {
          packages = with nixpkgs.legacyPackages.${system};
            [
              go_1_26
              just
              nodejs_24
              pnpm_10
            ]
            ++ pkgs.lib.optionals pkgs.stdenv.hostPlatform.isLinux [playwrightBrowsers];
          shellHook = pkgs.lib.optionalString pkgs.stdenv.hostPlatform.isLinux ''
            export PLAYWRIGHT_BROWSERS_PATH="${playwrightBrowsers}"
          '';
        };
    });
  };
}
