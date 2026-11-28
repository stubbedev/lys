# Developing lys

lys is one Go module, `github.com/stubbedev/lys`, in one repository. Each
component lives in its own directory (`lib/`, `ui/`, `desktop/`, `wm/`,
`shell/`, `login/`, `settings/`, `viewer/`, `files/`, `polkit/`, `power/`,
`secrets/`, `splash/`) with its own justfile, Nix modules and lint config. Everything is
driven through `just`; CI calls the same recipes.

## Dev shells

`nix develop` is the shell for the whole repository. Each component has its own,
with the tools and environment its tests need: `nix develop .#wm`,
`nix develop .#login`. Run a component's recipes in its shell from the root:

```sh
just in wm plain race        # recipes of wm/justfile, in .#wm
just in wm pace-gate vulkan  # a recipe with its arguments
```

Inside a shell, `just <component>::<recipe>` runs the same recipe:
`just login::check`. `just --list --list-submodules` lists every recipe.

## Repository recipes

| Recipe | Effect |
| --- | --- |
| `just gates [target]...` | Every Go gate and race suite of the targets (default every component) in parallel, each recipe in its own process in its component's dev shell, as many at once as cores and memory allow, over one Go build cache; prints each result, the failures' logs and the wall clock. A target is a component or one part of a component with `shards` (`wm/2`), with `:gates` for its Go gates alone or `:race` for its race suite alone, as CI's Go jobs name them: `just gates wm/3:race lib`. A component with `shards` named whole (`settings`, `wm:race`) runs as every part `ci/components.json` splits it into, planned by `ci/affected -gates` from the same definitions CI's jobs use, so no race suite runs as one package past Go's ten-minute package timeout. A component with `shards` has a last part for its static gates, the ones `static-gates` names (`ui/3` for ui's one shard). A part that splits a package by test name runs that package in a process of its own beside the part's other packages. A target whose justfile `just --fmt --check` refuses fails before anything runs. |
| `just checks <name>...` | Build Nix checks for this machine, named `<component>-<check>`: `just checks login-vms-greeter wm-module`. |
| `just vm-checks <name>...` | Run NixOS VM checks one after another on closures built first: the drivers, then each test alone. |
| `just build-stage <component>... [vm]...` | A CI build job for this machine: every package and non-VM check of the components, and the named VM checks' closures, in one `nix build` that keeps going past a failure. Their dev shells are `just build-shells`' work. |
| `just build-shells <component>...` | The CI job the Go, further and package jobs wait for: the components' dev shells on this machine. |
| `just packages <name>...` | Build flake packages: `just packages lys-wm`. |
| `just affected [base]` | The components and CI jobs a change since `base` (default `origin/main`) touches. |
| `just repository-check` | At the same time: a tidy `go.mod`, the CI helpers' tests and lint, `nix/go-packages.json` matching the tree, every flake output evaluating in nix-eval-jobs workers that share one 6 GiB budget, the `vms` lists of `ci/components.json` naming every VM check of the flake once, and each package hashing only its own sources (touching `docs/` changes no package, touching a Go package changes exactly its importers and what is built on them); prints each part's result and the wall clock. |
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
source. A package's build starts from the Go build caches of the code outside
lys, each compiled once per architecture and shared by every package:
`lys-go-build-cache-std`, the whole standard library, a derivation of the Go
toolchain alone, and `lys-go-build-cache-modules`, every third-party package a
lys package imports (the `external` and `testExternal` imports
`nix/go-packages.json` records) with its dependencies, a derivation of
`go.mod`, `go.sum` and that list, built on the standard library's. Each holds
only the entries it compiled, and a package's build links them into its
`GOCACHE`, the modules' only when it imports a third-party package, so no
package outside lys is compiled twice: with one cache per distinct import set
there were twelve, three of them compiling folio and four modernc.org/libc,
which took 97 s on twelve x86_64 cores against 15 s and 18 s for the two
layers. A change to lys's code rebuilds neither; the modules' cache, and with
it every package that imports a third-party package, rebuilds when `go.mod`,
`go.sum` or the set of third-party packages lys imports changes, and the
standard library's only with the toolchain. A cold build compiles only lys's
own packages on top of them: lys-shell's build phase takes 30 s against 63 s
without, with the same binaries. The flake exposes
`packages.lys-<component>`, `nixosModules.lys-<component>`,
`homeModules.lys-<component>`, `overlays.default`, one dev shell per
component, and every check as `checks.<system>.<component>-<check>`.
Components' VM scenarios that run on the same machine are subtests of one
NixOS test, one boot for all of them: `nix/desktop-vms.nix` gathers them into
`desktop-vms-session` (lys-shell on lys-wm, then the idle wakeup gate, each
in a fresh session of the same boot), `desktop-vms-screencast` (a screen
shared through lys-shell's portal) and
`desktop-vms-system` (KMS, lys-users, lys-secrets activation, lys-power
freezing a user's unit on battery, lys-polkit,
then the clock steps and suspend of `shell/internal/clockwatch`). The session
VM, the screencast VM and the system VM are a CI job each, since the lane's
longest job is its critical path: the screencast and system VMs in one job
took 6.6 minutes after the 2.9-minute build (193 s and 105 s of test script
after a minute of closure download), and apart about 4.5 and 3 minutes.

The idle wakeup gate (`nix/idle-vm.nix`) measures a still session of lys-wm,
lys-shell and lys-power with `lys-power energy` from the moment both say they
have started (lys-power's unit is active once it reports ready, and the
shell's Shell1 `Started` turns true once its bars show, their dropdowns are
built and its tray has synced): the first 5 s against a startup budget, then
the minute after in three windows against the idle one, with nothing waited
out between. It fails when one of them wakes more often, or keeps the GPU
busier, over a window than `nix/idle-budgets.json` allows, naming the process
and its busiest threads, and when lys-power or lys-shell restarted while it
measured. It then opens a client that redraws every frame in its own scope,
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

`ci/components.json` holds `components`, `sharded-gates`, `static-gates`,
`gate-runners`, `package-inputs` and `go-jobs`. Per component it says which recipes of its justfile are its Go gates
(`gates`), whether it has a race suite (`race`), how its race suite and the
gates `sharded-gates` names split into parts (`shards`), its
further jobs (each a `run` command on named runners, with `after-build` when it
uses its component's build rather than only a dev shell), its NixOS VM checks
(`vms`, a list of jobs, each a list of checks run in one job), and
what besides its Go imports it depends on (a VM test that boots another
component's package). `gate-runners` names the runners that run only some of
the Go gates, and which: aarch64 runs only the gates whose result depends on
the architecture (`test`, `build`, `test-lysdebug`, `atspi`,
`atspi-verify`, `dist`), and vet, lint, formatting, `no-fused`, `vuln` and
`schema-check` run on x86_64 alone; `ci/affected` refuses a gate no runner
runs, and `just gates` applies the same list on the machine it runs on.
x86_64 runs every gate but `test`: every component with a test gate but
polkit and splash has a race suite, which runs the same tests under the race
detector, so the plain run on x86_64 only repeated them, at 1009 s of the Go
jobs' recipe time in run 45974767264. Each test now runs once per
architecture, under `-race` on x86_64 and plainly on aarch64. The price is
that polkit's and splash's tests, which have no race suite, run on aarch64
alone, and that no CI job runs the other components' tests plainly on
x86_64.
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

The repository job also runs `ci/execsites`, which holds every process lys
starts at runtime to `ci/exec-allowlist.json`. It parses, with `go/parser`
alone, the Go packages the flake's packages ship (the import closure, from
`nix/go-packages.json`, of the main packages their `subPackages` name) and
finds every `os/exec` `Command`, `CommandContext`, `LookPath` and `Cmd`
literal, `os.StartProcess`, `syscall` `Exec`, `ForkExec` and `StartProcess`,
and `unix.Exec`, following import aliases and string constants to the program;
a program it cannot resolve is `dynamic`. A `go` entry names the package, the
enclosing function and the program. The same tool reads the Nix files the
flake's packages and modules are built from (neither checks nor dev shells)
and the systemd and D-Bus unit files for what they put on `PATH`
(`makeBinPath`, `--suffix`/`--prefix PATH`, `environment.systemPackages`,
`home.packages`, a unit's `path`) and the programs their `Exec*` lines run,
and a non-lys program among them needs a `packaging` entry naming the file and
the program. Every entry gives a `class` (`integration` for the user's
programs and those a protocol mandates, `own` for a lys program, `replace`
for one an issue removes, which `issue` names) and a `reason`. A start without
an entry fails at its line, and so does an entry nothing in the tree matches,
so the change that removes a start deletes its entry.

The account runs about twenty jobs at once, so a full run's wall clock is about
its runner-minutes divided by twenty, plus the longest lane's build, and
each job pays about a minute of fixed cost (runner, checkout, Nix install, the
dev shell). Jobs are therefore as few as fit: work is grouped until a job holds
about four to seven warm minutes of it, and split only where one suite alone
comes near ten. A runner has four cores, and `just gates` runs four recipes at
once, so a job whose recipes are each a few minutes long ends with its longest
one: lib's race suite (510 s) and its gates (test 377 s, lint 204 s) in one
job took 9.7 minutes, the last job of run 44897398937, so they run apart, the
race suite beside power and login's race suite; wm's light race parts, `wm/1`
and `wm/2` (91 to 117 s each), share a job with shell's third race part, the
harness's halves, `wm/3` and `wm/4` (254 and 230 s on four local cores), have
a job each, and so does `settings/2`, while `settings/1` shares one with
secrets, viewer and files.

`go-jobs` lists each runner's jobs longest first, and the plan lists every
runner's Go jobs before its further jobs and x86_64's before aarch64's, so the
longest work takes the first runners. The account takes jobs in the order they
queue, and the Go jobs queue together once the dev shells are built, before
the slower lane builds finish: in run 45974767264 wm's VM jobs were ready at
2.7 minutes but started at 7.0 to 7.7, behind the Go jobs, and ended the run at
13.4. So the Go stage runs at most fourteen jobs at once (`max-parallel` of
`run.yml`), the rest starting in the matrix's order as those finish, by which
time the VM jobs have queued ahead of them. A list-scheduling simulation of
that run's job times with twenty runners put the run at about 10.6 minutes
with the cap and 11.3 without it; caps from 12 to 15 all came to 10.6 to 11.0.
The cost is that a Go job past the fourteenth waits for a runner even when one
is free.

The twenty runners are the account's, not the run's. Run 46141152927
(4e16d9f50) took 17.2 minutes on 168 runner-minutes. It never had more than
13 jobs running at once, because the docs build (18 minutes on one runner) and
CodeQL (10 minutes) ran beside it. The login and desktop VM jobs were ready at
3.7 to 3.9 minutes but waited until 10.0 to 11.5. The docs and security
workflows on `main` therefore start once `ci` has finished (`workflow_run`), so
they no longer take its runners. The docs build runs only when something it
builds from changed since the last deployed build. Security still runs on
every pull request and every night. login's `login-vms-sessions` and
`login-vms-greetd`, 7.2 minutes as one job and the run's last, are a job each.
That run's work cannot finish in ten minutes even on twenty runners: the same
simulation, reproducing it at 17.3 minutes on twelve runners, gives 12.6 with
the cap on twenty, and a scheduler with perfect priority 11.1. Its cold builds
built `lys-login` in four jobs, `lys-settings` in three and `lys-shell` in two.

The jobs run in lanes, each a build and the jobs on it, none waiting for
another lane's build:

1. **Component lanes**, one per component with VM checks (on x86_64), the
   `lanes` of the plan, run by `.github/workflows/lane.yml`: the component's
   build job, `just build-stage wm wm-vms-desktop ...`, builds its packages,
   non-VM checks and VM closures, and its VM jobs and the further jobs it
   marks `after-build` start as soon as that build is done, not after the
   other components' builds: wm's VM jobs no longer wait for splash's build.
   No job frees the runner's disk: no check builds a disk image any more,
   and a cold build of every component's outputs uses about 4 GiB of the
   85 GiB a runner has free.

   No VM check builds a disk image of its system. `useBootLoader` copies the
   whole system closure, 1.4 to 3.8 GiB, into a qcow2 image that every change
   to a package in it rebuilds and the build job pushes whole: two of them
   made a splash Go change push 5.5 GiB, past the build job's push limit
   (#382), where the same change now pushes 294 MiB. splash's LUKS checks,
   which boot a specialisation that `switch-to-configuration boot`
   installs, boot directly instead: the host is their boot loader, the
   guest's `boot.loader.external` hook writes the toplevel it installs to
   the machine's `xchg` directory, and the run script boots that toplevel's
   kernel, initrd and `init`, as systemd-boot's entry would, under OVMF.
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
   six-minute build. It realises the environments side by side, one
   `nix print-dev-env` each, and names each failure with its log: one after
   another, they took 60 s of the job's 76 s build step in run 45974767264
   though every one was in the binary cache already, and every Go and
   further job waits for this job.

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
     suites (one job each, `just in lib gpu headless`), wm's Vulkan
     pacing gate (apart, since it is timing-sensitive and near five
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
     parts that runs something there, that names a suite or part a component
     lacks (the race suite has no static part) or a part that runs no gate on
     its runner, or a component whose `shards` no job uses, and a
     `static-gates` entry that is no component's gate or that `sharded-gates`
     also splits. A gates part runs nothing on a runner whose `gate-runners`
     leave it none of its gates: on x86_64, which runs no `test`, the gates
     parts of settings, shell and wm below their shards are empty, so their
     gates parts run nothing there, and a whole-component target such as
     `settings:gates` runs as its parts that have a gate on that runner;
     `just gates` names the part and the architecture when a target runs
     nothing. A push runs each Go job
     with only its affected targets, and skips it when none is; the job keeps
     its id, and with it its Go build cache, whatever subset runs.
   - **Shards**: a component one of whose suites would not fit a job lists
     `shards`, each a list of its package directories (`./cmd/lys-wm`,
     `./internal/...`) and of tests of one package by name
     (`{"package": "./internal/harness", "tests": ["TestA", "TestB"]}`), which
     splits a package too slow for one job. Part *k* of its race suite, and of
     each gate the top-level `sharded-gates` names (`test` and
     `test-lysdebug`), tests only shard *k*'s packages, and its named tests
     with `go test -run '^(TestA|TestB)$'`; the part after them, `wm/4` for wm's
     three shards, tests every package of `go list ./...` no shard names (with
     the recipe's build flags: `-race` for race, `-tags lysdebug` for
     test-lysdebug), every package split by test name with
     `-skip` of every test the shards name, and runs every other gate but the
     static ones, so no test is dropped when a package or a test is added. The
     static gates, those the top-level `static-gates` names (`fmt-check`,
     `vet`, `lint`, `no-fused`, `vuln`), read the whole tree rather than test
     packages, so they run once, whole, in a last part of their own, part
     *n*+2 of the Go gates of a component with *n* shards (`ui/3`, `shell/4`,
     `wm/5`), on a runner that runs one of them; the race suite stops at part
     *n*+1. A change to a package every one of a component's packages imports
     leaves those checks nothing cached: ui's took vet 222 s, lint 369 s and
     no-fused 224 s of one four-core job in run 45393011425, which with the
     tests beside them came to 602 s, so they now run beside the tests in a
     job of their own rather than after them. `go test` takes one
     `-run` for all its packages, so `just gates` runs a part's recipe once
     for its whole packages and once more for each package it splits, each in
     a process of its own, with the packages in `$LYS_TEST_PACKAGES` and the
     patterns in `$LYS_TEST_RUN` and `$LYS_TEST_SKIP`, which the sharded
     recipes of a component test instead of `./...` and every test. wm splits
     its race suite: the harness, about eight and a half minutes as one part,
     runs its 185 slowest tests in `wm/3` and the rest beside wm's other
     packages in `wm/4`. ui splits its test and lysdebug suites (`ui/1`, the
     client and app tests, `ui/2` the rest beside build and the at-spi gates,
     `ui/3` its static gates), of which x86_64 runs the lysdebug parts and
     aarch64 runs ui's gates whole, shell its test and race suites, of which
     CI runs only the race suite in parts, and settings its race suite: `./internal/settings` runs one test at a time, since the
     UI's font shaper and preview state are not goroutine-safe, and takes
     about five and a half minutes under `-race` on four local cores, so
     `settings/1` runs its six sweeps over every page or every row and
     `settings/2` the rest and every other package. A property every page
     holds is a `pageCheck` of the one sweep that builds and lays out every
     page once for all of them, not a test of its own that builds them all
     again. A new test
     joins the part after the shards, so when a part outgrows its job, regroup the named
     tests from `go test -json` times. `ci/affected` refuses a shard that names a directory
     without tests, a test its package does not declare, a name that is not
     a test function's or a test twice, a package both whole and by test
     name, and a sharded recipe that ignores one of the three variables, on
     every plan, not only when `ci/` changes.
   - Each Go job restores the newest Go build cache it saved, by its job id,
     and saves it at the end whether it passed, failed or timed out, under a
     key of this run, so the next run starts from this run's compiled packages,
     lint results and test results, not from the first run of the day's. It
     saves only the build and lint cache entries it used
     (`.github/actions/go-cache-save`), so a cache holds one job's working set
     rather than every entry of the last five days: the repository's caches
     had grown past the 10 GB GitHub keeps, one job's to 1.9 GB that took 90 s
     to restore, and the eviction left Go lib to start cold in run
     44897398937, without one cached test result. Once saved, the job
     deletes its older entries on the same ref (the workflows grant
     `actions: write` for it), so each job keeps one entry: two runs' worth
     came to 10.7 GB, and the eviction left settings/2:gates desktop to
     compile cold past its ten minutes in the run on 3e961121a. The
     downloaded modules are not in a job's entry: every Go job restores one
     shared entry of `~/go/pkg/mod/cache/download`, keyed on `go.sum`, and
     Go extracts what it imports from those zips. The plan job is its one
     writer: when `go.sum` has no entry yet it runs `go mod download` for
     every module, saves it and deletes the modules of an older `go.sum`
     (a Go job that saved it would hold only the modules its own packages
     need). Each job had saved the whole module cache, 496 MB unpacked,
     about thirty times over.
   - **package**: each packaged component's archive, `.deb` and `.rpm`, on
     the dev shells, for a component whose packages a change touches the
     inputs of: a path `package-inputs` names, `{component}` standing for
     its name (its `packaging/`, `resources/`, justfile and `LICENSE`, the
     shared `packaging/`, `package.yml`, `go.mod` and `go.sum`). A change
     to its Go code alone packages nothing: the Go jobs and its Nix build
     compile that code already, and the package jobs were about 24 of a
     full run's 213 runner-minutes. A run of every component (the weekly
     schedule, a manual run, a release) changes `go.mod` and so packages
     every component. Its `dist` recipe compiles the release binaries again
     rather than taking them from the Nix packages: those are linked for the
     Nix store, the archives for the distributions' loader, and the measured
     `dist` and `archive` step takes 15 to 75 seconds of a two to four
     minute job. Every `dist` compiles through `packaging::go-build`, with
     the flake's `dist-go`: go.dev's release of the Go version the dev
     shells pin, not nixpkgs' Go, whose standard library reads
     `/etc/services` and `/etc/protocols` from a `/nix/store` path instead
     of `/etc` and whose linker names a `/nix/store` loader. After the
     archive, the `.deb` and the `.rpm` are built, `packaging::store-check`
     fails the job when one of them holds no ELF file or any file naming a
     `/nix/store` path, and prints each such file with the paths it names;
     `packaging::store-check-test`, run before it, checks the check.
     `packaging::build` runs `debian/rules binary` under
     `dpkg-architecture` rather than `dpkg-buildpackage`, which builds the
     same `.deb` and then spent 35 to 88 seconds of each job in
     `dpkg-genbuildinfo` over the runner's thousands of installed packages,
     for a `.buildinfo` and `.changes` nothing uploads.

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
