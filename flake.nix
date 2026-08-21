{
  description = "Development environment for rsp-website";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = {nixpkgs, ...}: let
    systems = ["aarch64-darwin" "aarch64-linux" "x86_64-linux"];
    forEachSystem = nixpkgs.lib.genAttrs systems;
  in {
    devShells = forEachSystem (system: {
      default = nixpkgs.legacyPackages.${system}.mkShell {
        packages = with nixpkgs.legacyPackages.${system}; [
          go_1_26
          just
          nodejs_24
          pnpm_10
        ];
      };
    });
  };
}
