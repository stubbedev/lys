# AGENTS.md

lys is one Go module in one repository; `README.md` says what each component
directory holds and `docs/development/index.md` how the repository is built,
tested and released. A component's own `AGENTS.md` or `CLAUDE.md` (`settings/`,
`wm/`, `shell/`) adds what is particular to it.

## Gates

`just affected` names the components a change touches, through their files,
their Go imports and the dependencies `ci/components.json` declares. Before
every commit, run their gates the way CI does, through `just`:

```sh
just in <component> check     # or the component's plain/race/docker recipes
just packages lys-<component> # the Nix package, for a component that has one
just repository-check         # after a change to go.mod, the flake, nix/ or ci/
```

Tests that start a compositor, a bus or a session run in a container
(`just <component>::docker <recipe>`), a NixOS VM (`just checks <name>`) or CI,
never against the desktop session on the host.

After a `go.mod` change, refresh the one `vendorHash` with `just vendor-hash`.
A new component, or a new CI job, is an entry in `ci/components.json`.

Verify locally before pushing, push, and check CI once when pushes stop. A red
`main` is fixed before anything else lands.

## CI and gates finish in ten minutes

A full CI run, and any local gate run, finishes in under ten minutes of wall
clock. A gate that takes longer is a bug to fix, never a wait to sit through.

- **Build once, share everywhere.** Every package, test binary and VM closure
  is built once per architecture and reused through the binary cache by every
  job and every test that needs it. No job rebuilds what another already built.
- **Share VMs.** VM tests that can run on the same machine run in one booted
  VM, as subtests of one NixOS test, not one boot per scenario.
- **Parallel, never serial.** Independent checks run at the same time, in CI
  as jobs and locally as concurrent processes. Nothing runs component by
  component in a line.
- **Measure it.** A change that makes a job slower says by how much and why,
  and a job that grows past its share of the ten minutes is split or made
  cheaper in the same change.

## Tests define behavior

Every feature and every bug fix lands with tests, positive and negative: a
behavior holds when it should, and when it should not, the failure is the
defined one. A fix without a test that fails before it is not a fix. Gates count
work, never wall clock; timeouts only stop hangs.

## Releases

Every component is released under one version: `just release-patch` tags a
commit CI passed on. Nothing in the repository pins another part of it, so there
is nothing to bump.

## The tracker

GitHub issues on `stubbedev/lys`. Nothing is planned in local files.
