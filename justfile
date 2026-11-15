# lys: every component's recipes as a module (`just login::check`), and the
# recipes that act on the repository as a whole.

set shell := ["bash", "-euo", "pipefail", "-c"]

mod lib
mod ui
mod desktop
mod wm
mod shell
mod login
mod settings
mod polkit
mod secrets
mod splash
mod packaging

# List the recipes, the components' included.
default:
    @just --list --list-submodules

# Run component recipes in the component's dev shell, as CI does:
# `just in wm plain race`, `just in wm pace-gate gles`.
in component +args:
    nix develop .#{{ component }} -c just --justfile {{ component }}/justfile --working-directory {{ component }} {{ args }}

# Build Nix checks for this machine's system, one after another so each VM test
# has the machine to itself: `just checks login-vm wm-module`.
[script("bash")]
checks +names:
    set -euo pipefail
    for name in {{ names }}; do
        nix build -L --no-link ".#checks.{{ arch() }}-linux.${name}"
    done

# Build flake packages: `just packages lys-wm lys-shell`.
[script("bash")]
packages +names:
    set -euo pipefail
    attrs=()
    for name in {{ names }}; do attrs+=(".#${name}"); done
    nix build -L --no-link "${attrs[@]}"

# The components and CI jobs a change since base touches, imports included.
affected base="origin/main":
    go run ./ci/affected -base {{ base }}

# The repository's own gate: a tidy go.mod, the CI planner's tests and lint, and
# every package, dev shell and check evaluating on both systems, each in its own
# process so no evaluation holds them all at once.
[script("bash")]
repository-check:
    set -euo pipefail
    nix develop .#lib -c go mod tidy -diff
    nix develop .#lib -c go test ./ci/...
    nix develop .#lib -c golangci-lint run --config lib/.golangci.yml ./ci/...
    for system in x86_64-linux aarch64-linux; do
        for output in packages devShells checks; do
            for name in $(nix eval --raw ".#${output}.${system}" --apply 'set: builtins.concatStringsSep " " (builtins.attrNames set)'); do
                nix eval --raw ".#${output}.${system}.${name}.drvPath" >/dev/null
            done
        done
    done
    nix eval --raw .#nixosModules --apply 'm: builtins.concatStringsSep " " (builtins.attrNames m)' >/dev/null

# Fail, showing the diff, when go.mod or go.sum is not tidy.
tidy-check:
    go mod tidy -diff

# Update the Go dependencies to their latest versions, then the vendor hash.
update:
    go get -u ./...
    go mod tidy
    just vendor-hash

# Set nix/go.nix's vendorHash to the hash of the modules go.sum pins.
vendor-hash:
    nix run github:stubbedev/lys-ci#vendor-hash -- nix/go.nix .#lys-polkit.goModules

# A release tag's workflow builds every component's packages. Each recipe tags
# the current commit, refusing anything but a clean default branch CI passed.

# Tag the current commit as the next patch release and push the tag.
release-patch:
    nix run github:stubbedev/lys-ci#release -- patch

# Tag the current commit as the next minor release and push the tag.
release-minor:
    nix run github:stubbedev/lys-ci#release -- minor

# Tag the current commit as the next major release and push the tag.
release-major:
    nix run github:stubbedev/lys-ci#release -- major
