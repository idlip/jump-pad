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
              go gopls gotools gofumpt
              sqlite
              just
              prettier treefmt
            ];

            shellHook = ''
              echo "jump-pad dev shell — go: $(go version)"
            '';
          };
        });

      packages = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };

          # jump-pad's only dependency is modernc.org/sqlite
          mkRelease = { goos, goarch }:
            pkgs.buildGoModule {
              pname = "jump-pad";
              version = "0.1.0";
              src = ./.;
              vendorHash = "sha256-5WaCZ29wuU/aP05IBHTM0WhELYrYoerGlIS3QxoXL5o=";
              subPackages = [ "cmd/jump-pad" ];
              env = {
                GOOS = goos;
                GOARCH = goarch;
                CGO_ENABLED = "0";
              };

              meta = {
                description = "Minimal URL shortener and pastebin, one Go binary";
                license = pkgs.lib.licenses.mit;
                mainProgram = "jump-pad";
              };
            };
        in
        {
          default = pkgs.buildGoModule {
            pname = "jump-pad";
            version = "0.1.0";
            src = ./.;
            vendorHash = "sha256-5WaCZ29wuU/aP05IBHTM0WhELYrYoerGlIS3QxoXL5o=";
            subPackages = [ "cmd/jump-pad" ];

            meta = {
              description = "Minimal URL shortener and pastebin, one Go binary";
              license = pkgs.lib.licenses.mit;
              mainProgram = "jump-pad";
            };
          };

          release-linux-amd64 = mkRelease { goos = "linux"; goarch = "amd64"; };
          release-linux-arm64 = mkRelease { goos = "linux"; goarch = "arm64"; };
          release-darwin-amd64 = mkRelease { goos = "darwin"; goarch = "amd64"; };
          release-darwin-arm64 = mkRelease { goos = "darwin"; goarch = "arm64"; };
          release-windows-amd64 = mkRelease { goos = "windows"; goarch = "amd64"; };
          release-windows-arm64 = mkRelease { goos = "windows"; goarch = "arm64"; };
        });

    };
}
