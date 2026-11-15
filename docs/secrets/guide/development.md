# Development

## Go packages

| Package | Contents | Used by |
| --- | --- | --- |
| `vault` | `dev.stubbe.lys.Vault1` contract, daemon and client | lys-shell's vault widget, launcher mode and network agent |
| `client` | Secret Service client: lookup, store, delete, browse collections and items, `ChangeLock`; plain (`Open`) or DH (`OpenEncrypted`) session | lys-shell's Secret portal, lys-settings' passwords page, the vault's session store |
| `control` | Unlock socket client (`Unlock`, `Rekey`) and server | lys-login |
| `transport` | The `plain` and DH secret transports | `client`, the service |
| `secretbus` | The Secret Service's D-Bus interfaces, events and signals | `client`, the service |
| `secretstest` | The real Secret Service on a test bus, with scripted password prompts | Consumers' tests |

Everything else is under `internal/`.

## Recipes

| Recipe | Effect |
| --- | --- |
| `just check` | `vet`, `lint`, `test`, `race`, `build` in order |
| `just vet` | `go vet ./...` |
| `just lint` | `golangci-lint fmt --diff` and `golangci-lint run` (`.golangci.yml`) |
| `just test` | `go test -shuffle=on ./...` |
| `just race` | Tests under the race detector |
| `just build` | Compile every package |
| `just dist` | Static, stripped release binary in `dist/` |
| `just fmt` | Format all Go sources |

`nix develop .#secrets` provides the pinned Go toolchain, the lint
tools, dbus for the tests' private buses, and the clients tested against
(secret-tool, python-secretstorage, Chromium, git's libsecret helper).

`just checks secrets-package secrets-hm-module secrets-nixos-module secrets-activation-vm`
builds the package, evaluates both modules and boots a NixOS VM that checks
D-Bus activation under dbus-broker. Repository-wide recipes, the Nix layout, CI and releases are in [Developing lys](/development/).

## Releases

lys-secrets is released with every other lys app, under one version
([Releases](/install/#releases)).
