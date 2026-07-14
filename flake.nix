{
  description = "jump-pad dev environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/d407951447dcd00442e97087bf374aad70c04cea";
  };

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              nim
              nimble
              sqlite
              pkg-config
            ];

            shellHook = ''
              echo "jump-pad dev shell — nim: $(nim --version | head -n1)"
            '';
          };
        });

      # `packages` (needed for `nix build` / `nix run` and the release
      # binary) comes once an actual .nimble file + source exist — add a
      # `nimPackages.buildNimPackage` derivation per-system here, forAllSystems
      # style like devShells above (see docs/roadmap.org — deployment
      # targets: raw binary / systemd / Nix package).
    };
}
