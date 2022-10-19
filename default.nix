# This Nix expression enables installation from a URL or local checkout of Hive:
#
# From URL: nix-env -if https://github.com/threatgrid/nix-hive/archive/main.tar.gz
# From the current directory: nix-env -if .
# 
# There is also an overlay in nix/overlay.nix, a simple package in nix/package.nix and a nix shell in shell.nix.
#
# Note that this uses https://github.com/edolstra/flake-compat to pass emulate Nix flakes, even on Nix releases without flake support.
(import
  (
    let lock = builtins.fromJSON (builtins.readFile ./flake.lock);
    in
    fetchTarball {
      url =
        "https://github.com/edolstra/flake-compat/archive/${lock.nodes.flake-compat.locked.rev}.tar.gz";
      sha256 = lock.nodes.flake-compat.locked.narHash;
    }
  )
  { src = ./.; }
).defaultNix
