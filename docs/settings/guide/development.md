# Development

The flake provides every dev tool and the `lys-wm` the tests run against:
or `nix develop .#settings`.

## Recipes

| Recipe | What it does |
| --- | --- |
| `just docker [recipe]` | Run a recipe (default `check`) in a container away from the desktop session |
| `just check` | `vet`, `lint`, `no-fused`, `test`, `race`, `build`, in order |
| `just vet` | `go vet` |
| `just lint` | `golangci-lint` formatting check and lints |
| `just fmt` | Format every Go source |
| `just no-fused` | Fail on float products fused into an FMA on arm64 |
| `just test` | The tests, including the end-to-end test against a headless lys-wm |
| `just race` | The tests under the race detector |
| `just build` | Compile-check every package |
| `just dist` | The release binary, into `dist/` |
| `just run [args]` | Run the app |

`just test`, `just race` and `just check` start a headless lys-wm: run them through
`just docker`, never against the desktop session. A plain `go test ./...` is safe:
the end-to-end and contract tests skip without `LYS_SETTINGS_HEADLESS`.

## Tests

- `internal/wmconfig` tests byte-level edits, with a fuzz target, and the override
  file's failure paths against a fake `lys-wm`: refused, read-only, a Home Manager
  config, a dangling link, changed during the check, reload failed.
- `internal/settings` tests the pages and rows offscreen, including that every
  control writes only what lys-shell or lys-wm accepts, and that no row falls back
  to a plain text editor unless it is on the free-form list.
- `e2e` starts a headless lys-wm in a private runtime directory, changes its
  config, reads the result back over IPC, and opens the real window on it.

## Nix

`just packages lys-settings` builds the package. `just checks settings-hm-module
settings-wrapper-vulkan` adds the Home Manager module check and the wrapper and
desktop-file check. accounts-daemon's rename (`nix/accounts-vm.nix`) is a
scenario of the shared NixOS VM `just checks desktop-vms-system`. Repository-wide recipes, the Nix layout, CI and releases are in [Developing lys](/development/).
`nix fmt` formats Nix files with alejandra.

## Releases

lys-settings is released with every other lys app, under one version
([Releases](/install/#releases)).
