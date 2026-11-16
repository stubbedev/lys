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
mod power
mod secrets
mod splash
mod packaging

# List the recipes, the components' included.
default:
    @just --list --list-submodules

# Run component recipes in the component's dev shell, as CI does:
# `just in wm plain race`, `just in wm pace-gate gles`.
in component +args:
    nix develop .#{{ component }} -c just --justfile {{ component }}/justfile --working-directory {{ component }} {{ args }}

# Build Nix checks for this machine's system, one after another so each VM test
# has the machine to itself: `just checks login-vms-greeter wm-module`.
[script("bash")]
checks +names:
    set -euo pipefail
    for name in {{ names }}; do
        nix build -L --no-link ".#checks.{{ arch() }}-linux.${name}"
    done

# Run NixOS VM checks, one after another, on closures built beforehand: their
# drivers first, which CI's max-jobs = 0 only lets come from the binary cache
# their components' build jobs filled, then each test alone: `just vm-checks login-vms-greeter`.
[script("bash")]
vm-checks +names:
    set -euo pipefail
    drivers=()
    for name in {{ names }}; do drivers+=(".#checks.{{ arch() }}-linux.${name}.driver"); done
    if ! nix build -L --no-link "${drivers[@]}"; then
        echo "vm-checks: the closures of {{ names }} neither built nor came from a substituter; in CI their component's build job builds and pushes them" >&2
        exit 1
    fi
    NIX_CONFIG="max-jobs = 1" just checks {{ names }}

# A CI build job for this machine's system, in one nix build that keeps going
# past a failure: every package, non-VM check and dev shell of one component,
# and the closures of the VM checks named after it, which its CI jobs run. A
# flake output belongs to the component its name starts with, the longest such
# name winning: `just build-stage wm wm-vms-desktop wm-vms-gpus`.
[script("bash")]
build-stage component *vms:
    set -euo pipefail
    component={{ quote(component) }}
    read -ra vms <<< {{ quote(vms) }}
    mapfile -t components < <(jq -r 'keys[]' ci/components.json)
    if [[ ! " ${components[*]} " == *" ${component} "* ]]; then
        echo "build-stage: ${component} is no component of ci/components.json" >&2
        exit 1
    fi
    for name in "${vms[@]}"; do
        if [[ ! "$name" =~ ^${component}-[a-z0-9-]+$ ]]; then
            echo "build-stage: ${name} is no check name of ${component}" >&2
            exit 1
        fi
    done
    list() { for name; do printf '"%s" ' "$name"; done; }
    dir=$(mktemp -d)
    trap 'rm -rf "$dir"' EXIT
    just eval-jobs "flake: let
        lib = flake.inputs.nixpkgs.lib;
        system = \"{{ arch() }}-linux\";
        component = \"${component}\";
        owners = name: lib.filter (c: name == c || lib.hasPrefix \"\${c}-\" name) [ $(list "${components[@]}") ];
        owned = name: lib.foldl' (longest: c: if lib.stringLength c > lib.stringLength longest then c else longest) \"\" (owners name) == component;
      in {
        packages = lib.filterAttrs (name: _: lib.hasPrefix \"lys-\" name && owned (lib.removePrefix \"lys-\" name)) flake.outputs.packages.\${system};
        checks = lib.filterAttrs (name: check: owned name && !(check ? driver)) flake.outputs.checks.\${system};
        devShells = lib.filterAttrs (name: _: name == component) flake.outputs.devShells.\${system};
        vms = lib.genAttrs [ $(list "${vms[@]}") ] (name: flake.outputs.checks.\${system}.\${name}.driver);
      }" > "${dir}/jobs"
    mapfile -t drvs < <(jq -r '.drvPath + "^*"' "${dir}/jobs")
    if (( ${#drvs[@]} == 0 )); then
        echo "build-stage: ${component} has nothing to build on {{ arch() }}-linux" >&2
        exit 1
    fi
    nix build -L --keep-going --no-link "${drvs[@]}"
    for shell in $(jq -r 'select(.attr | startswith("devShells.")) | .attr | ltrimstr("devShells.")' "${dir}/jobs"); do
        nix print-dev-env --profile "${dir}/${shell}" ".#${shell}" >/dev/null
    done

# Print the valid store paths, derivations and sources aside, missing from a
# listing of the store taken before: what a CI build job built or substituted,
# for the binary cache, also when its build failed or timed out part way.
[private]
[script("bash")]
new-store-paths listing:
    set -euo pipefail
    declare -A before=() invalid=()
    while read -r name; do before[$name]=1; done < {{ quote(listing) }}
    paths=()
    for path in /nix/store/*; do
        name=${path#/nix/store/}
        case "$name" in
            *.drv | *.lock | *.chroot | *-source) ;;
            *) [[ -v before[$name] ]] || paths+=("$path") ;;
        esac
    done
    (( ${#paths[@]} )) || exit 0
    while read -r path; do invalid[$path]=1; done < <(nix-store --check-validity --print-invalid "${paths[@]}")
    for path in "${paths[@]}"; do
        [[ -v invalid[$path] ]] || printf '%s\n' "$path"
    done

# Evaluate the derivations a nix-eval-jobs selection of the flake picks, on as
# many workers as cores and memory allow, and print them as JSON lines; any
# evaluation error fails it, naming the attribute.
[private]
[script("bash")]
eval-jobs select:
    set -euo pipefail
    memory=$(awk '/^MemTotal:/ { print int($2 / 1048576) }' /proc/meminfo)
    workers=$(( memory / 4 < $(nproc) ? memory / 4 : $(nproc) ))
    out=$(mktemp)
    trap 'rm -f "$out"' EXIT
    nix run --inputs-from . nixpkgs#nix-eval-jobs -- --flake . --force-recurse --workers "$(( workers > 0 ? workers : 1 ))" --max-memory-size 3072 --select {{ quote(select) }} > "$out"
    if jq -e -s 'any(.[]; has("error") and .error != null)' "$out" >/dev/null; then
        jq -r 'select(.error != null) | "eval-jobs: \(.attr) does not evaluate:\n\(.error)"' "$out" >&2
        exit 1
    fi
    cat "$out"

# Run the Go gates and race suites of the components (every one when none is
# named) in parallel, each gate recipe in its own process in its component's
# dev shell, sharing one Go build cache, as many at once as cores and memory
# allow, and print the wall clock: `just gates`, `just gates wm shell`. A
# component with `shards` in ci/components.json splits its test and race suites
# into one part per shard and a last part for every other package; `wm/2` runs
# only the second part's test and race suites, and the last part, `wm/3` for a
# wm of two shards, runs the rest of them and every other gate.
gates *targets: (run-gates "gates race" targets)

# The Go gates alone, as CI's Go jobs run them on each architecture.
go-gates *targets: (run-gates "gates" targets)

# The race suites alone, as CI's race jobs run them.
race-gates *targets: (run-gates "race" targets)

[private]
[script("bash")]
run-gates kinds targets:
    set -euo pipefail
    start=$SECONDS
    read -ra kinds <<< {{ quote(kinds) }}
    read -ra targets <<< {{ quote(targets) }}
    if (( ${#targets[@]} == 0 )); then
        mapfile -t targets < <(jq -r 'to_entries[] | select(.value.gates or .value.race) | .key' ci/components.json)
    fi
    tasks=()
    declare -A shard=() rest=()
    for target in "${targets[@]}"; do
        c=${target%%/*}
        part=${target#"$c"}
        part=${part#/}
        if ! jq -e --arg c "$c" 'has($c)' ci/components.json >/dev/null; then
            echo "gates: ${c} is no component of ci/components.json" >&2
            exit 1
        fi
        shards=$(jq -r --arg c "$c" '.[$c].shards // [] | length' ci/components.json)
        if [[ "$target" == */* ]] && ! [[ "$part" =~ ^[1-9][0-9]*$ && "$part" -le $((shards + 1)) ]]; then
            echo "gates: ${target} is no part of ${c}, which has parts 1 to $((shards + 1))" >&2
            exit 1
        fi
        for kind in "${kinds[@]}"; do
            while read -r recipe; do
                task="${target} ${recipe}"
                tasks+=("$task")
                if [[ "$recipe" != test && "$recipe" != race ]] || [[ -z "$part" ]] || (( shards == 0 )); then continue; fi
                if (( part <= shards )); then
                    shard[$task]=$(jq -r --arg c "$c" --argjson i "$((part - 1))" '.[$c].shards[$i] | join(" ")' ci/components.json)
                else
                    rest[$task]=$(jq -r --arg c "$c" '[.[$c].shards[][]] | join(" ")' ci/components.json)
                fi
            done < <(jq -r --arg c "$c" --arg kind "$kind" --argjson part "${part:-0}" '
                .[$c] | (.shards // [] | length) as $n
                | if $kind == "race" then (select(.race) | "race") else (.gates // [])[] end
                | select($part == 0 or $part > $n or . == "test" or . == "race")
            ' ci/components.json)
        done
    done
    if (( ${#tasks[@]} == 0 )); then
        echo "gates: ${targets[*]} run no ${kinds[*]}" >&2
        exit 1
    fi
    dir=$(mktemp -d)
    trap 'jobs -pr | xargs -r kill 2>/dev/null; wait; rm -rf "$dir"' EXIT
    declare -A running=() started=() envs=()
    mapfile -t components < <(printf '%s\n' "${tasks[@]}" | cut -d' ' -f1 | cut -d/ -f1 | sort -u)
    for c in "${components[@]}"; do
        envs[$c]="${dir}/env-${#envs[@]}"
        nix print-dev-env ".#${c}" > "${envs[$c]}" 2> "${envs[$c]}.log" &
        running[$!]=$c
    done
    for pid in "${!running[@]}"; do
        if ! wait "$pid"; then
            echo "gates: the ${running[$pid]} dev shell did not evaluate:" >&2
            cat "${envs[${running[$pid]}]}.log" >&2
            exit 1
        fi
    done
    running=()
    for task in "${!rest[@]}"; do
        read -r target recipe <<< "$task"
        c=${target%%/*}
        read -ra sharded <<< "${rest[$task]}"
        # shellcheck source=/dev/null
        if ! (
            set +euo pipefail
            source "${envs[$c]}"
            set -euo pipefail
            cd "$c"
            build=()
            if [[ "$recipe" == race ]]; then export CGO_ENABLED=1; build=(-race); fi
            go list "${build[@]}" ./... | sort > "${dir}/all"
            go list "${build[@]}" "${sharded[@]}" | sort > "${dir}/sharded"
        ) 2> "${dir}/list.log"; then
            echo "gates: the packages of ${target} ${recipe} outside ${c}'s shards do not list:" >&2
            cat "${dir}/list.log" >&2
            exit 1
        fi
        rest[$task]=$(comm -23 "${dir}/all" "${dir}/sharded" | paste -sd' ')
        if [[ -z "${rest[$task]}" ]]; then
            echo "gates: ${c}'s shards hold every package, so ${target} ${recipe} would test none" >&2
            exit 1
        fi
    done
    if [[ " ${kinds[*]} " == *" gates "* ]] && (( ${#tasks[@]} > 1 )); then
        trees=()
        for c in "${components[@]}"; do trees+=("./${c}/..."); done
        # shellcheck source=/dev/null
        if ! (set +euo pipefail; source "${envs[${components[0]}]}" && set -e && go build "${trees[@]}") > "${dir}/build.log" 2>&1; then
            echo "gates: ${components[*]} do not compile:" >&2
            cat "${dir}/build.log" >&2
            exit 1
        fi
    fi
    memory=$(awk '/^MemAvailable:/ { print int($2 / 3145728) }' /proc/meminfo)
    slots=$(( memory < $(nproc) ? memory : $(nproc) ))
    slots=$(( slots > 0 ? slots : 1 ))
    mapfile -t tasks < <(printf '%s\n' "${tasks[@]}" | awk '{ print ($2 == "race" ? 0 : $2 == "lint" ? 1 : $2 ~ /^test/ ? 2 : 3), $0 }' | sort -s -k1,1n | cut -d' ' -f2-)
    echo "gates: ready in $((SECONDS - start))s, running ${#tasks[@]} recipes, ${slots} at a time"
    log() { printf '%s/%s.log' "$dir" "${1//[\/ ]/-}"; }
    failed=()
    reap() {
        local pid status=0 task
        wait -n -p pid "${!running[@]}" || status=$?
        task=${running[$pid]}
        unset "running[$pid]"
        if (( status == 0 )); then
            printf 'ok    %-26s %4ss\n' "$task" "$((SECONDS - started[$pid]))"
        else
            printf 'FAIL  %-26s %4ss  exit %s\n' "$task" "$((SECONDS - started[$pid]))" "$status"
            failed+=("$task")
        fi
    }
    for task in "${tasks[@]}"; do
        while (( ${#running[@]} >= slots )); do reap; done
        read -r target recipe <<< "$task"
        c=${target%%/*}
        packages="${shard[$task]-${rest[$task]-}}"
        (
            set +euo pipefail
            # shellcheck source=/dev/null
            source "${envs[$c]}"
            if [[ -n "$packages" ]]; then export LYS_TEST_PACKAGES="$packages"; fi
            exec just --justfile "${c}/justfile" --working-directory "$c" "$recipe"
        ) > "$(log "$task")" 2>&1 &
        running[$!]=$task
        started[$!]=$SECONDS
    done
    while (( ${#running[@]} )); do reap; done
    if [[ "${GITHUB_ACTIONS-}" == true ]]; then
        for task in "${tasks[@]}"; do
            echo "::group::${task}"
            cat "$(log "$task")"
            echo "::endgroup::"
        done
    fi
    for task in "${failed[@]}"; do
        printf '\n===== %s =====\n' "$task"
        cat "$(log "$task")"
    done
    printf '\ngates: %d recipes, %d failed, %ss wall clock\n' "${#tasks[@]}" "${#failed[@]}" "$((SECONDS - start))"
    (( ${#failed[@]} == 0 ))

# Build flake packages: `just packages lys-wm lys-shell`.
[script("bash")]
packages +names:
    set -euo pipefail
    attrs=()
    for name in {{ names }}; do attrs+=(".#${name}"); done
    nix build -L --no-link "${attrs[@]}"

# The components and CI jobs a change since base touches, imports included.
affected base="origin/main":
    go run ./ci/affected -base {{ base }}

# The repository's own gate, its parts at the same time: a tidy go.mod, the CI
# helpers' tests and lint, nix/go-packages.json matching the tree, every
# package, dev shell and check evaluating on both systems, spread over
# nix-eval-jobs' workers so no evaluation holds them all at once, the NixOS
# modules evaluating, the VM jobs covering every VM check, and each package
# hashing only its own sources. Prints each part's result and the wall clock.
[script("bash")]
repository-check:
    set -euo pipefail
    start=$SECONDS
    dir=$(mktemp -d)
    trap 'jobs -pr | xargs -r kill 2>/dev/null; wait; rm -rf "$dir"' EXIT
    declare -A parts=()
    nix develop .#lib -c bash -euo pipefail -c 'go mod tidy -diff && go test ./ci/... && golangci-lint run --config lib/.golangci.yml ./ci/... && go run ./ci/gopackages -check nix/go-packages.json' > "${dir}/go" 2>&1 &
    parts[$!]=go
    # shellcheck disable=SC2016
    just eval-jobs 'flake: flake.inputs.nixpkgs.lib.genAttrs [ "x86_64-linux" "aarch64-linux" ] (system: { packages = flake.outputs.packages.${system}; devShells = flake.outputs.devShells.${system}; checks = flake.outputs.checks.${system}; })' > /dev/null 2> "${dir}/outputs" &
    parts[$!]=outputs
    nix eval --raw .#nixosModules --apply 'm: builtins.concatStringsSep " " (builtins.attrNames m)' > /dev/null 2> "${dir}/modules" &
    parts[$!]=modules
    just vm-coverage > "${dir}/vm-coverage" 2>&1 &
    parts[$!]=vm-coverage
    nix develop .#lib -c just source-check > "${dir}/sources" 2>&1 &
    parts[$!]=sources
    failed=0
    while (( ${#parts[@]} )); do
        status=0
        wait -n -p pid "${!parts[@]}" || status=$?
        part=${parts[$pid]}
        unset "parts[$pid]"
        if (( status == 0 )); then
            printf 'ok    %-12s %4ss\n' "$part" "$((SECONDS - start))"
            continue
        fi
        printf 'FAIL  %-12s %4ss  exit %s\n' "$part" "$((SECONDS - start))" "$status"
        cat "${dir}/${part}"
        failed=1
    done
    printf 'repository-check: %ss wall clock\n' "$((SECONDS - start))"
    (( failed == 0 ))

# Fail unless the VM jobs of ci/components.json run every NixOS VM check of the
# flake on x86_64-linux, each once, and name no check the flake lacks.
[private]
[script("bash")]
vm-coverage:
    set -euo pipefail
    # shellcheck disable=SC2016
    flake=$(nix eval --json .#checks.x86_64-linux --apply 'checks: builtins.filter (name: checks.${name} ? driver) (builtins.attrNames checks)')
    problems=$(jq -r -n --argjson flake "$flake" --slurpfile components ci/components.json '
        [$components[0][] | .vms[]?] as $ci
        | ($flake - $ci | map("vm-coverage: the flake VM check \(.) is in no VM job of ci/components.json"))
        + ($ci - $flake | map("vm-coverage: ci/components.json runs \(.), which is no VM check of the flake on x86_64-linux"))
        + ($ci | group_by(.) | map(select(length > 1) | "vm-coverage: ci/components.json runs \(.[0]) in \(length) jobs"))
        | .[]
    ')
    if [[ -n "$problems" ]]; then
        printf '%s\n' "$problems" >&2
        exit 1
    fi

# Fail unless every package's derivation hashes only what it builds from, in a
# copy of the tree: touching every file under docs/ changes no package, and
# touching a Go package changes exactly the packages whose Go targets import it
# (as go list sees them) and the packages built on those.
[private]
[script("bash")]
source-check:
    set -euo pipefail
    system=$(nix eval --impure --raw --expr builtins.currentSystem)
    tree=$(mktemp -d)
    trap 'rm -rf "$tree"' EXIT
    rev=$(git stash create)
    git archive "${rev:-HEAD}" | tar -x -C "$tree"
    cd "$tree"
    git init -q
    git add -A
    export CGO_ENABLED=0
    drvs() { nix eval --json ".#packages.${system}" --apply 'builtins.mapAttrs (_: p: p.drvPath)'; }
    # shellcheck disable=SC2016
    targets=$(nix eval --json ".#packages.${system}" --apply 'ps: builtins.mapAttrs (_: p: { packages = (p.subPackages or [ ]) ++ p.goPackages; tests = p.goTests; }) (builtins.removeAttrs ps (builtins.filter (name: !(ps.${name} ? goTests)) (builtins.attrNames ps)))')
    declare -A deps=()
    for attr in $(jq -r 'keys[]' <<< "$targets"); do
        mapfile -t packages < <(jq -r --arg attr "$attr" '.[$attr].packages[] | "./" + .' <<< "$targets")
        mapfile -t tests < <(jq -r --arg attr "$attr" '.[$attr].tests[] | "./" + .' <<< "$targets")
        list=""
        if (( ${#packages[@]} )); then list+=$(go list -deps -f '{{{{.Dir}}' "${packages[@]}")$'\n'; fi
        if (( ${#tests[@]} )); then list+=$(go list -deps -test -f '{{{{.Dir}}' "${tests[@]}"); fi
        deps[$attr]=$(sort -u <<< "$list")
    done
    changed() { jq -r -n --argjson before "$1" --argjson after "$2" '[$before | keys[] | select($before[.] != $after[.])] | join(" ")'; }
    before=$(drvs)
    mapfile -d '' docs < <(git ls-files -z docs)
    for file in "${docs[@]}"; do printf '\n' >> "$file"; done
    after=$(drvs)
    failed=0
    if [[ -n "$(changed "$before" "$after")" ]]; then
        printf 'source-check: touching docs/ changed %s\n' "$(changed "$before" "$after")" >&2
        failed=1
    fi
    for package in polkit/cmd/lys-polkit desktop/i18n; do
        before=$after
        mapfile -t files < <(git ls-files -- "$package" | grep -E "^${package}/[^/]+\.go$" | grep -v '_test\.go$')
        if (( ! ${#files[@]} )); then
            printf 'source-check: %s holds no Go file to touch\n' "$package" >&2
            exit 1
        fi
        file=${files[0]}
        printf '\n' >> "$file"
        after=$(drvs)
        importers=()
        for attr in "${!deps[@]}"; do
            if grep -qxF "${tree}/${package}" <<< "${deps[$attr]}"; then importers+=("$(jq -r --arg attr "$attr" '.[$attr]' <<< "$before")"); fi
        done
        want=()
        for attr in $(jq -r 'keys[]' <<< "$before"); do
            requisites=$(nix-store --query --requisites "$(jq -r --arg attr "$attr" '.[$attr]' <<< "$before")")
            for drv in "${importers[@]}"; do
                if grep -qxF "$drv" <<< "$requisites"; then want+=("$attr"); break; fi
            done
        done
        got=$(changed "$before" "$after")
        if [[ -z "$got" || "$got" != "${want[*]}" ]]; then
            printf 'source-check: touching %s changed [%s], want [%s]\n' "$file" "$got" "${want[*]}" >&2
            failed=1
        fi
    done
    (( failed == 0 ))

# Write nix/go-packages.json: each Go package's imports and embedded files within
# the module, which the packages' filtered sources in nix/go.nix follow.
go-packages:
    go run ./ci/gopackages nix/go-packages.json

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
