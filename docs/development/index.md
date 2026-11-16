# Developing lys

lys is one Go module, `github.com/stubbedev/lys`, in one repository. Each
component lives in its own directory (`lib/`, `ui/`, `desktop/`, `wm/`,
`shell/`, `login/`, `settings/`, `polkit/`, `power/`, `secrets/`, `splash/`) with its
own justfile, Nix modules and lint config. Everything is driven through
`just`; CI calls the same recipes.

## Dev shells

`nix develop` is the shell for the whole repository. Each component has its own,
with the tools and environment its tests need: `nix develop .#wm`,
`nix develop .#login`. Run a component's recipes in its shell from the root:

```sh
just in wm plain race        # recipes of wm/justfile, in .#wm
just in wm pace-gate gles    # a recipe with its arguments
```

Inside a shell, `just <component>::<recipe>` runs the same recipe:
`just login::check`. `just --list --list-submodules` lists every recipe.

## Repository recipes

| Recipe | Effect |
| --- | --- |
| `just gates [component]...` | Every Go gate and race suite of the components (default all) in parallel, each recipe in its own process in its component's dev shell, as many at once as cores and memory allow, over one Go build cache; prints each result, the failures' logs and the wall clock. |
| `just go-gates [target]...`, `just race-gates [target]...` | The Go gates alone, or the race suites alone, as CI's Go and race jobs run them. A target is a component, or one part of a component with `shards`: `just race-gates wm/2`. |
| `just checks <name>...` | Build Nix checks for this machine, named `<component>-<check>`: `just checks login-vms-greeter wm-module`. |
| `just vm-checks <name>...` | Run NixOS VM checks one after another on closures built first: the drivers, then each test alone. |
| `just build-stage <component> [vm]...` | A CI build job for this machine: every package, non-VM check and dev shell of the component, and the named VM checks' closures, in one `nix build` that keeps going past a failure. |
| `just packages <name>...` | Build flake packages: `just packages lys-wm`. |
| `just affected [base]` | The components and CI jobs a change since `base` (default `origin/main`) touches. |
| `just repository-check` | At the same time: a tidy `go.mod`, the CI helpers' tests and lint, `nix/go-packages.json` matching the tree, every flake output evaluating, spread over nix-eval-jobs workers, the `vms` lists of `ci/components.json` naming every VM check of the flake once, and each package hashing only its own sources (touching `docs/` changes no package, touching a Go package changes exactly its importers and what is built on them); prints each part's result and the wall clock. |
| `just go-packages` | Rewrite `nix/go-packages.json` after a change to the imports, the embedded files or the set of Go packages. |
| `just update` | Update every Go dependency, then the vendor hash. |
| `just vendor-hash` | Recompute the one `vendorHash` in `nix/go.nix` after a `go.mod` change. |
| `just release-patch`, `release-minor`, `release-major` | Tag the next release once CI passed on the commit. |

## Nix

The root flake builds every component from one source tree with one
`vendorHash`, so a `go.mod` change needs one `just vendor-hash`. The hash
covers the module cache `go.mod` and `go.sum` pin, and nothing else of the
tree. Each package's source is only what it builds from: `go.mod`, `go.sum`,
the Go packages its `subPackages` and `goPackages` import (with the test files
and test imports of its `goTests`), the files they embed, and the `resources`
its install steps read, all repository paths. `lysGo.build` in `nix/go.nix`
assembles it from `nix/go-packages.json`, which `ci/gopackages` writes from
the tracked Go files, so a commit rebuilds only the packages, and the VM images
on them, whose sources it touched. A package's version is the hash of that
source. The flake exposes
`packages.lys-<component>`, `nixosModules.lys-<component>`,
`homeModules.lys-<component>`, `overlays.default`, one dev shell per
component, and every check as `checks.<system>.<component>-<check>`.
Components' VM scenarios that run on the same machine are subtests of one
NixOS test, one boot for all of them: `nix/desktop-vms.nix` gathers them into
`desktop-vms-session` (lys-shell on lys-wm, then a screen shared through
lys-shell's portal, each in a fresh session of the same boot) and
`desktop-vms-system` (KMS, accounts-daemon, lys-secrets activation, lys-polkit,
then the clock steps and suspend of `shell/internal/clockwatch`), a CI job each.

## CI

Every CI job, and every local gate run, finishes in under ten minutes of wall
clock; a job that grows past that is split or made cheaper, never given a longer
timeout. Every job's `timeout-minutes` is 10.

`ci/components.json` says, per component, which recipes of its justfile are its
Go gates (`gates`), whether it has a race suite (`race`), how its test and race
suites split over jobs (`shards`), its further jobs (each a `run` command on
named runners), its NixOS VM checks (`vms`, with `free-disk` when their disk
images need the runner's preinstalled toolchains gone), and what besides its Go
imports it depends on (a VM test that boots another component's package). On
every push and pull request the plan job runs `ci/affected`, which takes the
changed files, adds every component importing or depending on a changed one, and
plans only their work. A change to the shared files (`go.mod`, the flake, `nix/`,
`ci/`, the root justfile, the CI workflow and its setup action) plans every
component, as do the weekly scheduled run and a manual one. A component's
`exclude` takes single files back out of its `paths`: the repository component
excludes `nix/go-packages.json`, which every change to the in-module imports
rewrites, so such a change plans only the components whose Go files it touched
and their importers, as the Go import analysis already finds them.

The jobs run in two stages:

1. **Build**, one job per component and architecture: `just build-stage
   <component>` builds, in one `nix build`, the component's packages, non-VM
   checks and dev shell, and on x86_64 the closures of its VM checks. A
   component builds on every architecture when it has Go gates, since the Go
   gates job of each architecture runs in its dev shell, and on x86_64 when it
   has VM checks; a component with neither (`repository`, `packaging`) builds
   nothing. The build step stops after six minutes, and the next step pushes
   every store path that appeared during the job to the binary cache
   (`nix.stubbe.dev`, through `XILO_TOKEN`; without the token the job fails,
   since nothing could be shared), whether the build passed, failed or timed
   out, so a re-run continues from what was built instead of starting over.
   Derivations two components share, such as `lys-wm` in the VM closures of
   wm, login and splash, are built by each of their jobs on a cold cache, side
   by side, and substituted once one of them pushed.
2. **Jobs**, each on the build jobs' outputs, once every build job finished.
   Their Nix runs with `max-jobs = 0`: it only substitutes, so a job that
   would build a lys derivation fails instead of building it again.
   - **Go gates**, one job per affected component and architecture: `just
     go-gates <component>`, each gate recipe in its own process in the
     component's dev shell, as many at once as cores and memory allow, after
     one `go build ./<component>/...` fills the job's Go build cache.
   - **race**, one job per affected component on x86_64: `just race-gates
     <component>`.
   - **Shards**: a component whose suites would not fit ten minutes in one job
     lists `shards` in `ci/components.json`, each a list of its package
     directories (`./cmd/lys-wm`, `./internal/...`). Its `test` gate and its
     race suite then run as one job per shard plus one last job, `wm/1` to
     `wm/4` for wm's three shards: job *k* tests only shard *k*'s packages, and
     the last job tests every package of `go list ./...` no shard names (with
     the race suite's build flags for race) and runs every other gate, so no
     test is dropped when a package is added. The parts reach the recipe as
     `$LYS_TEST_PACKAGES`, which the `test` and `race` recipes of a sharded
     component test instead of `./...`; `just gates <component>` leaves it
     unset and runs the whole suite in one go. A shard is a group of the
     packages slowest under the race detector, chosen from CI's timings;
     `ci/affected`'s tests fail when a shard names a directory without tests
     or a sharded recipe ignores `$LYS_TEST_PACKAGES`.
   - Each Go job restores its own Go build cache, keyed by its job id, and saves
     it at the end whether it passed, failed or timed out, so the next run
     starts warm.
   - **Further jobs**, each a job of its own so they run side by side: lib's
     and ui's GPU and headless suites, wm's GLES and Vulkan pacing gates,
     splash's initramfs-tools image, and one job per VM check, which
     runs `just vm-checks <check>` on the closure its component's build job
     built, so the VM checks run side by side. The `vms` lists in
     `ci/components.json` are the one place the checks are named. VM jobs run
     on x86_64, the only runner with KVM, and `just repository-check` fails
     unless the lists hold every VM check the flake has there, each exactly
     once, and none it lacks.
   - **package**: each packaged component's archive, `.deb` and `.rpm`.

The Go test suites run in the Go gates and race jobs only. The packages build
without a `checkPhase`, since it would run the same tests again in each build
job; Nix checks run what the gates cannot, such as `splash-plymouth-client`,
the one test that needs nixpkgs' plymouth.

The `ci` job is the one status to require: it passes when every planned job
passed.

A new component needs an entry in `ci/components.json`; a test fails until it
has one, and another until every gate it names is a recipe of its justfile.

## Releases

Every component is released together under one version. `just release-patch`
tags the commit once CI passed on it, and the tag's workflow builds each
packaged component's archive, `.deb` and `.rpm` through its justfile
(`archive`, `smoke-test`, and the shared `packaging::build`) and publishes them
on one GitHub release. Binaries report the version through
`lib/buildinfo.Version`: the tag a checkout was built from, the version
`go install …@vX.Y.Z` records, or the one Nix stamps.
