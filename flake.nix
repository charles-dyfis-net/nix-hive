{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/release-22.05";
    flake-compat.url = "github:edolstra/flake-compat";
    flake-compat.flake = false;
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs = { self, nixpkgs, flake-utils, ... }:
    {
      overlays = {
        default = final: prev: {
          hive = prev.callPackage ./nix/package.nix { };
        };
      };
    } // flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        packages = rec {
          default = hive;
          hive = pkgs.callPackage ./nix/package.nix { inherit pkgs system; };
        };
        devShells = {
          default = pkgs.mkShell {
            name = "nix-hive-shell";
            buildInputs = with pkgs; [ go nixpkgs-fmt nix ];
            shellHook = ''
              # We inject <hive> into your path to support local development.
              export NIX_PATH="hive=$PWD/nix/hive:$NIX_PATH"

              nixfmtAll() { find . -name '*.nix' -exec nixpkgs-fmt -- {} +; }
            '';
          };
        };
      });
}
