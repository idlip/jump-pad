# Quick commands. `just --list` to see all of these.

# Build the binary into ./jump-pad
build:
    go build -o jump-pad ./cmd/jump-pad

# Build+run against a scratch DB, for live manual testing
dev:
    go run ./cmd/jump-pad -addr :8080 -db dev.db

# Run the test suite
test:
    go test ./...

# Pure Nix build (see flake.nix packages.default)
nix-build:
    nix build .#default

# Cross-compile every release binary into ./dist, for a GitHub release
release:
    mkdir -p dist
    for target in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64 windows-arm64; do \
        nix build .#release-$target -o dist/result-$target; \
        if [ -f dist/result-$target/bin/jump-pad.exe ]; then \
            cp dist/result-$target/bin/jump-pad.exe dist/jump-pad-$target.exe; \
        else \
            cp dist/result-$target/bin/jump-pad dist/jump-pad-$target; \
        fi; \
    done
    rm -rf dist/result-*

# Remove build/dev artifacts
clean:
    rm -f jump-pad dev.db dev.db-wal dev.db-shm result
    rm -rf dist

# Regenerate docs/api.org from the route table
apidocs:
    go run ./cmd/apidocs
