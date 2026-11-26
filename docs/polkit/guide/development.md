# Development

The flake's dev shell provides Go (the `go.mod` version), gopls,
golangci-lint, delve, just, git, gh and dbus, with `CGO_ENABLED=0` and
`GOTOOLCHAIN=local`. Enter it with `nix develop .#polkit`.

D-Bus goes through [lys-lib](/lib/)'s godbus fork and `dbusx`; the dialog
contract is lys-lib's `portaldialogs`.

## Recipes

| Recipe | Effect |
| --- | --- |
| `just check` | `vet`, `lint`, `test`, `build` in order |
| `just vet` | `go vet ./...` |
| `just lint` | `golangci-lint run` with `.golangci.yml` |
| `just test` | `go test ./...` |
| `just build` | Compile-checks every package |
| `just fmt` | `gofmt -w .` |

## Tests

`go test` needs no polkitd, logind or shell: fakes of the polkit Authority,
logind and the dialogs serve on private dbus-daemons (lys-lib's `dbustest`),
and the test binary acts as `polkit-agent-helper-1` over both socket and
setuid transports.

## Nix

| Command | Effect |
| --- | --- |
| `just packages lys-polkit` | Builds the package with the user unit in `lib/systemd/user` (`nix/lys-polkit.nix`) |
| `just checks polkit-hm-module` | The home-manager module's unit wiring |
| `just checks desktop-vms-system` | On x86_64, the shared NixOS VM whose scenarios include `tests/polkit-vm.nix` |
| `nix fmt` | Formats nix files with alejandra |

Repository-wide recipes, the Nix layout, CI and releases are in [Developing lys](/development/).

The VM test runs `pkexec` through lys-polkit against a real polkitd, with
`internal/vmtest/fakedialogs` scripting the dialog: right password, details,
a name-service-only user, wrong then right password, cancel, the setuid helper
without the socket, `auth_admin_keep` listing/reuse/revoke/expiry, and no
agent after lys-polkit stops.

## Linting

`polkit/.golangci.yml`: standard linters plus bodyclose, copyloopvar, errorlint,
gosec, intrange, misspell, modernize, nolintlint, perfsprint, revive,
unconvert, unparam, usestdlibvars and whitespace; formatters gofumpt and gci.
No issue caps.
