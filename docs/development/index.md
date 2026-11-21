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
| `just gates [target]...` | Every Go gate and race suite of the targets (default every component) in parallel, each recipe in its own process in its component's dev shell, as many at once as cores and memory allow, over one Go build cache; prints each result, the failures' logs and the wall clock. A target is a component or one part of a component with `shards` (`wm/2`), with `:gates` for its Go gates alone or `:race` for its race suite alone, as CI's Go jobs name them: `just gates wm/3:race lib`. |
| `just checks <name>...` | Build Nix checks for this machine, named `<component>-<check>`: `just checks login-vms-greeter wm-module`. |
| `just vm-checks <name>...` | Run NixOS VM checks one after another on closures built first: the drivers, then each test alone. |
| `just build-stage <component>... [vm]...` | A CI build job for this machine: every package and non-VM check of the components, and the named VM checks' closures, in one `nix build` that keeps going past a failure. Their dev shells are `just build-shells`' work. |
| `just build-shells <component>...` | The CI job the Go, further and package jobs wait for: the components' dev shells on this machine. |
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
source. A package's build starts from a Go build cache that holds the
standard library and third-party packages its Go packages import, compiled
once: `lys-go-build-cache`, a derivation of `go.mod`, `go.sum` and that list
of imports, which `nix/go-packages.json` records per package (`external`,
`testExternal`). It changes only with the modules or the package's set of
outside imports, so the binary cache serves it, and a cold build compiles
only lys's own packages: lys-shell's build phase takes 30 s on top of it
against 63 s without, with the same binaries. The flake exposes
`packages.lys-<component>`, `nixosModules.lys-<component>`,
`homeModules.lys-<component>`, `overlays.default`, one dev shell per
component, and every check as `checks.<system>.<component>-<check>`.
Components' VM scenarios that run on the same machine are subtests of one
NixOS test, one boot for all of them: `nix/desktop-vms.nix` gathers them into
`desktop-vms-session` (lys-shell on lys-wm, the idle wakeup gate, then a
screen shared through lys-shell's portal, each in a fresh session of the same
boot) and
`desktop-vms-system` (KMS, accounts-daemon, lys-secrets activation, lys-power
freezing a user's unit on battery, lys-polkit,
then the clock steps and suspend of `shell/internal/clockwatch`), a CI job each.

The idle wakeup gate (`nix/idle-vm.nix`) measures a still session of lys-wm,
lys-shell and lys-power with `lys-power energy` from the moment the shell's
bars are on screen: the first 5 s against a startup budget, then the minute
after in three windows against the idle one, with nothing waited out between.
It fails when one of them wakes more often, or keeps the GPU busier, over a
window than `nix/idle-budgets.json` allows, naming the process and its busiest
threads. It then opens a client that redraws every frame in its own scope,
hides it on another workspace, and fails unless lys-wm puts it to sleep, the
shell naps its scope (CPU and I/O weight 20, 50 ms timer slack), the client
wakes at most `hidden_client_wakeups` times over the window while lys-wm
stays within its budget, and showing it again restores all of it. Last it
gives the shell a clock that shows seconds and fails unless the gate catches
it.

## CI

Every CI job, and every local gate run, finishes in under ten minutes of wall
clock; a job that grows past that is split or made cheaper, never given a longer
timeout. Every job's `timeout-minutes` is 10.

`ci/components.json` holds `components`, `gate-runners` and `go-jobs`. Per component it says
which recipes of its justfile are its Go gates (`gates`), whether it has a race
suite (`race`), how its test and race suites split into parts (`shards`), its
further jobs (each a `run` command on named runners, with `after-build` when it
uses its component's build rather than only a dev shell), its NixOS VM checks
(`vms`, a list of jobs, each a list of checks run in one job, with `free-disk`
when their disk images need the runner's preinstalled toolchains gone), and
what besides its Go imports it depends on (a VM test that boots another
component's package). `gate-runners` names the runners that run only some of
the Go gates, and which: aarch64 runs only the gates whose result depends on
the architecture (`vet`, `test`, `build`, `test-lysdebug`, `atspi`,
`atspi-verify`, `dist`), and lint, formatting, `no-fused`, `vuln` and
`schema-check` run on x86_64 alone; `ci/affected` refuses a gate no runner
runs, and `just gates` applies the same list on the machine it runs on.
`go-jobs` lists, per runner, the Go jobs: each a list of
targets run together by one `just gates`. On every push and pull request the
plan job runs `ci/affected`, which takes the changed files, adds every
component importing or depending on a changed one, and plans only their work.
A change to the shared files (`go.mod`, the flake, `nix/`, `ci/`, the root
justfile, the CI workflow and its setup action) plans every component, as do
the weekly scheduled run and a manual one, with one exception: the repository
component's `go-packages` names `nix/go-packages.json`, which every change to
the in-module imports rewrites. A change to it plans the repository job, which
checks the file against the tree, and, from the entries that differ between the
file at the merge base and at `HEAD`, the components owning the changed package
directories and their importers, since those entries decide what their Nix
packages build from. A regeneration that changes no component's entry plans
only the repository job.

The account runs about twenty jobs at once, so a full run's wall clock is about
its runner-minutes divided by twenty, plus the longest lane's build, and
each job pays about a minute of fixed cost (runner, checkout, Nix install, the
dev shell). Jobs are therefore as few as fit: work is grouped until a job holds
six to eight minutes of it, and split only where one suite alone comes near ten.

The jobs run in lanes, each a build and the jobs on it, none waiting for
another lane's build:

1. **Component lanes**, one per component with VM checks (on x86_64), the
   `lanes` of the plan, run by `.github/workflows/lane.yml`: the component's
   build job, `just build-stage wm wm-vms-desktop ...`, builds its packages,
   non-VM checks and VM closures, first removing the runner's
   preinstalled toolchains only when the component sets `free-disk`, and its
   VM jobs and the further jobs it marks `after-build` start as soon as that
   build is done, not after the other components' builds: wm's VM jobs no
   longer wait for splash's disk images.
2. **Shared build**, per architecture: every other component with Go suites
   builds in one job, `just build-stage lib ui ...`, its packages and non-VM
   checks. A component with neither (`repository`, `packaging`)
   builds nothing. No job waits for it; the `ci` job does.
3. **Dev shells**, per architecture: one job, `just build-shells lib ui ...`,
   builds the dev shells of the components with Go suites or packages there,
   which is all the Go jobs, the other further jobs and the package jobs use,
   so they start a minute or so after the plan instead of after the slowest
   component build. It is the only job that builds them: no VM or
   `after-build` job uses a dev shell, so a build job fetching the gigabyte
   or more of a dev shell's inputs again would only take time from its own
   six-minute build.

A build job builds only the jobs nix-eval-jobs does not find in a binary cache (`--check-cache-status`), so outputs an earlier run already pushed, VM disk images included, are neither built nor downloaded again. Every build job pushes each store path to the binary cache (`xilos.dev`,
through `XILO_TOKEN`; without the token the job fails, since nothing could be
shared) as the build finishes it, with `xilo watch`, so uploads overlap the
build. The build stops after six minutes, and the next step pushes whatever
the watch has not, whether the build passed, failed or timed out; uploads are
chunked and deduplicated, so a re-run continues from what was built and
uploaded instead of starting over. Derivations two
VM components share, such as `lys-wm` in the VM closures of wm, login and
splash, are built by each of their jobs on a cold cache, side by side, and
substituted once one of them pushed. Both kinds of build job run
`.github/workflows/build.yml`, and every later job but the package jobs runs
`.github/workflows/run.yml`, whose Nix runs with `max-jobs = 0`: it only
substitutes, so a job that would build a lys derivation fails instead of
building it again. A lane lists its VM jobs longest first, so they take the
first runners. The jobs on the builds:

   - **VM jobs**, one per entry of a component's `vms`:
     `just vm-checks <check>...` on the closures its component's build job
     built. Checks share a job when they share most of a closure and still
     fit; a check
     near ten minutes is split into checks of fewer scenarios, each still one
     boot for all of its scenarios (`wm-vms-desktop` and `wm-vms-apps` were
     one check of about nine minutes). The `vms`
     lists are the one place the checks are named. VM jobs run on x86_64, the
     only runner with KVM, and `just repository-check` fails unless the lists
     hold every VM check the flake has there, each exactly once, and none it
     lacks.
   - **Further jobs**, each a `run` command: lib's and ui's GPU and headless
     suites (one job each, `just in lib gpu headless`), wm's GLES and Vulkan
     pacing gates (apart, since each is timing-sensitive and near five
     minutes) and the repository check, on the dev shells, and splash's
     initramfs-tools image, which needs `lys-splash`, after splash's build.
   - **Go jobs**, one per entry of `go-jobs`: `just gates <target>...`, each
     gate recipe in its own process in its component's dev shell, as many at
     once as cores and memory allow, after one `go build` of the job's
     components fills its Go build cache. A target is a component, or a part
     `<component>/<k>` of one with `shards`, with `:gates` or `:race` to run
     only that suite; a bare target runs every suite the component has on that
     runner, and race suites run on x86_64 only, so the plan writes a bare
     target as `<component>:gates` on aarch64. `ci/affected` refuses a
     `go-jobs` that does not run each component's Go gates on every runner and
     its race suite on x86_64 exactly once, either whole or in every one of its
     parts, that names a suite or part a component lacks, or a component whose
     `shards` no job uses. A push runs each Go job with only its affected
     targets, and skips it when none is; the job keeps its id, and with it its
     Go build cache, whatever subset runs.
   - **Shards**: a component one of whose suites would not fit a job lists
     `shards`, each a list of its package directories (`./cmd/lys-wm`,
     `./internal/...`). Part *k* of its test gate or race suite tests only shard
     *k*'s packages, and the last part, `wm/4` for wm's three shards, tests
     every package of `go list ./...` no shard names (with the race suite's
     build flags for race) and runs every other gate, so no test is dropped
     when a package is added. The parts reach the recipe as
     `$LYS_TEST_PACKAGES`, which the `test` and `race` recipes of a sharded
     component test instead of `./...`. wm splits only its race suite
     (`wm/3:race`, the harness, takes about eight minutes alone) and shell both
     suites; `ci/affected`'s tests fail when a shard names a directory without
     tests or a sharded recipe ignores `$LYS_TEST_PACKAGES`.
   - Each Go job restores its own Go build cache, keyed by its job id, and saves
     it at the end whether it passed, failed or timed out, so the next run
     starts warm.
   - **package**: each packaged component's archive, `.deb` and `.rpm`, on
     the dev shells. Its `dist` recipe compiles the release binaries again
     rather than taking them from the Nix packages: those are linked for the
     Nix store, the archives for the distributions' loader, and the measured
     `dist` and `archive` step takes 25 to 85 seconds of a two to three
     minute job, most of the rest being apt.

To regroup, edit `go-jobs` or a component's `vms` from the job times of a recent
full run (`gh run view <run> --json jobs`); `go run ./ci/affected` prints the
full plan.

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
