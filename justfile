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

# Remove build/dev artifacts
clean:
    rm -f jump-pad dev.db dev.db-wal dev.db-shm result