# Development

The flake's dev shell (`nix develop .#splash`) provides Go, gopls,
golangci-lint, delve, just, git, gh and the lys-ci tools, with
`CGO_ENABLED=0` and `GOTOOLCHAIN=local`.

## Layout

Paths are under `splash/` in the repository.

| Path | Contents |
| --- | --- |
| `cmd/lys-splash` | Role selection; starts the daemon or the client |
| `internal/protocol` | plymouthd's socket protocol: request and response codec |
| `internal/server` | Daemon side of the socket: peer credentials, deferred answers |
| `internal/client` | The plymouth command line |
| `internal/daemon` | Splash lifecycle from boot to quit, modes, boot duration |
| `internal/theme` | `theme.yaml` validation (defaults from `desktop/tokens`), frame rendering with damage, fades |
| `internal/kms` | Outputs on two dumb buffers per card: mode and scale, rotation, paced damage-limited flips, uevents |
| `internal/console` | Virtual terminal, captured console output, text fallback |
| `internal/key` | Terminal bytes decoded into keys |
| `internal/keymap` | Console keymap from `vconsole.conf` or `default/keyboard` |
| `internal/ask` | Prompt queue, typed text, cached passwords, keystroke watches |
| `internal/kmsg` | Log handler writing to `/dev/kmsg` for the detached daemon |
| `internal/asynclog` | Log handler that never blocks the caller |
| `units/` | systemd units with `@placeholders@` filled in by the package |
| `initramfs-tools/` | Hook and scripts for Ubuntu and Debian |
| `nix/` | Package, NixOS module, dev shell, checks |

## Recipes

| Recipe | Effect |
| --- | --- |
| `just check` | All gates in order: vet, lint, no-fused, test, build |
| `just vet` | `go vet ./...` |
| `just lint` | `golangci-lint run` with `.golangci.yml` |
| `just no-fused` | Fails on float products fused into FMA, which round differently from amd64 |
| `just test` | `go test ./...` |
| `just build` | Compile-checks every package |
| `just fmt` | `gofmt -w .` |
| `just plymouth-client` | Runs the `plymouth-client` check |
| `just initramfs-tools [image]` | Builds an initramfs-tools image from the compat output in an Ubuntu container (default `ubuntu:24.04`) and checks it holds the binary, links and scripts in order; needs Docker. It does not boot the image |

## Nix

- `just packages lys-splash`: the package (`nix/package.nix`), outputs `out` and `compat`.
- `nix fmt`: alejandra.
- `just checks splash-<name>` runs each of these, from the repository root. Repository-wide recipes, the Nix layout, CI and releases are in [Developing lys](/development/).

| Check | Tests |
| --- | --- |
| `package` | Builds the package and runs the unit tests |
| `plymouth-client` | nixpkgs' `plymouth` client against the server |
| `units` | Every unit conflicts with its Plymouth unit; units and initramfs-tools script agree on a table of kernel command lines |
| `module` | Evaluates the NixOS module and finds the splash in the initrd |
| `vms-greeter` | VMs on virtio-gpu, without a display and on vkms, booted at once, their scenarios in turn, each failure naming its scenario: `boot`, a boot through the module on virtio-gpu and with no display; `greeter`, hand-over to lys-login's greeter on lys-wm, checking every screen dump: the splash fades its group out, and the greeter's first frame holds the logo exactly where the splash left it while its screen fades in; on one vkms VM with two outputs, one turned, `kms`, the kms package's real-card test as root, then `greeter-outputs`, hand-over to the greeter with both outputs lit by the splash and both taken by the greeter, then `journal`, reading the guest's whole journal through a slowly drained VM shell |
| `vms-firmware` | OVMF VMs: `e2e`, the full path, each stage failing by name: OVMF and simpledrm, virtio-gpu, a LUKS root and a second LUKS disk unlocked by one passphrase at the splash, switch root, the plymouth client, hand-over to the greeter, the reboot splash, and a display-less machine typing its passphrase at the text fallback; on one more VM, `kms-switch`, the splash on simpledrm, then on the native driver as it takes over, then `daemon`, the daemon on virtio-gpu with the plymouth client and systemd's password agent, then detached as `plymouthd` logging to the journal |

The grid, the lys logo, the BGRT logo's placement and the record of the last
frame are `desktop/splashframe`, outside the splash, because lys-login's greeter
computes its first frame from them too.

virtio-gpu and vkms have no Vulkan driver, so `nix/greeter.nix` sets
`LYS_WM_RENDERER=gles` in the session variables, which reach the greeter's
lys-wm through PAM: it renders with GLES on llvmpipe instead of stopping.

Expected frames live in `internal/theme/testdata/golden`; this manual's
pictures of the splash are those files.
