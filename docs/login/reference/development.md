# Development

The code is under `login/`. `nix develop .#login` gives the dev shell: the pinned Go toolchain, the lint stack, fonts, and the headless sway and
private D-Bus the tests use.

| Recipe | Effect |
| --- | --- |
| `just check` | Release gates in order: vet, lint, no-fused, test, race, build. |
| `just vet` | `go vet ./...` |
| `just lint` | `golangci-lint fmt --diff` and `golangci-lint run` (`.golangci.yml`). |
| `just no-fused` | Fail on float products fused into FMA. |
| `just test [args]` | `go test -shuffle=on ./...` |
| `just race` | Tests under the race detector. |
| `just build` | Compile every package. |
| `just fmt` | Format all Go sources (gofumpt, gci). |
| `just dist` | Release binary in `dist/`, pure Go, with the standard loader and the wordlist notice. |
| `just distro-vm <debian\|fedora>` | Build, install and log in through the `.deb` or `.rpm` in a cloud VM (KVM, guest network). |

NixOS VM checks (KVM), each `just <name>`. A check boots one VM and runs its
scenarios on it in turn, resetting the display manager in between; a failure
names its scenario.

| Check | Scenarios |
| --- | --- |
| `vms-greeter` | `login`: daemon, greeter in lys-wm, wrong and right password, logind session, lys-secrets unlock, greeter returns. `accessibility`: accessibility menu, Super+Alt+S with Orca, persistence. `x11_session`: X11 session registered as `x11`, greeter as `wayland`. `keyboard_german`, `keyboard_switching`: system layout, layout switching, per-user layout. `keyring`, `no_keyring`: gnome-keyring started and unlocked without its PAM module; warnings for a rejected password and no keyring. |
| `vms-sessions` | `lock`: lock screen status row, layout switch, fprintd match and mismatch. `rekey`: a changed password re-keys the login collection. `switch`: fast user switching, upgrade with two sessions, logout, stop. `upgrade`: in-place upgrade with a session and lock screen; refused and failed upgrades. |
| `vms-greetd` | `lavapipe`: greeter under greetd in the lys-wm kiosk of `lys-login kiosk`, on lavapipe. `hidden`: a system account left out of the list but typed in. `deadkey`: a dead key in the password. `wallclock`: setting the wall clock updates the login clock. |
| `vms-autologin` | `autologin`: autologin, greeter after logout, LUKS passphrase opening the keyring. |

Each check is also `just checks login-<name>` from the repository root. Repository-wide recipes, the Nix layout, CI and releases are in [Developing lys](/development/).

The VM checks show the greeter on virtio-gpu, which has no Vulkan driver, so
`nix/tests/vm.nix` sets `LYS_WM_RENDERER=lavapipe` in the session variables
for the greeter's lys-wm and the lys-wm sessions.

The greeter logs `greeter: ready: ...` once its first frame is on screen and it
holds keyboard focus on a form field. VM checks wait for that line before typing.
