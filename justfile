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

# Build Nix checks for this machine's system: `just checks login-vm wm-module`.
[script("bash")]
checks +names:
    set -euo pipefail
    attrs=()
    for name in {{ names }}; do attrs+=(".#checks.{{ arch() }}-linux.${name}"); done
    nix build -L --no-link "${attrs[@]}"

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

# The repository's own gate: a tidy go.mod, the CI planner's tests and every
# flake output evaluating on both systems.
repository-check:
    nix develop .#lib -c go mod tidy -diff
    nix develop .#lib -c go test ./ci/...
    nix flake check --no-build --all-systems

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
