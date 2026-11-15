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

# List the recipes, the components' included.
default:
    @just --list --list-submodules

# Run one component's recipe in its dev shell, as CI does: `just in login check`.
in component recipe="check":
    nix develop .#{{ component }} -c just {{ component }}::{{ recipe }}

# The components and CI jobs a change since base touches, imports included.
affected base="origin/main":
    go run ./ci/affected -base {{ base }}

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
