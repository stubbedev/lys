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
# `just in wm plain race`, `just in wm pace-gate vulkan`.
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
# past a failure: every package and non-VM check of the named components, and
# the closures of the named VM checks of theirs, which their CI jobs run. Their
# dev shells are `just build-shells`' work, which no job on a build uses. A
# flake output belongs to the component its name starts with, the longest such
# name winning: `just build-stage lib ui`, `just build-stage wm wm-vms-desktop
# wm-vms-gpus`.
[script("bash")]
build-stage +names:
    set -euo pipefail
    read -ra names <<< {{ quote(names) }}
    mapfile -t components < <(jq -r '.components | keys[]' ci/components.json)
    selected=()
    vms=()
    for name in "${names[@]}"; do
        if [[ " ${components[*]} " == *" ${name} "* ]]; then
            selected+=("$name")
        else
            vms+=("$name")
        fi
    done
    if (( ${#selected[@]} == 0 )); then
        echo "build-stage: ${names[*]} name no component of ci/components.json" >&2
        exit 1
    fi
    for name in "${vms[@]}"; do
        if ! jq -e --arg vm "$name" '[.components[$ARGS.positional[]].vms[]?[]] | index($vm)' ci/components.json --args "${selected[@]}" >/dev/null; then
            echo "build-stage: ${name} is neither a component of ci/components.json nor a VM check of ${selected[*]}" >&2
            exit 1
        fi
    done
    list() { for name; do printf '"%s" ' "$name"; done; }
    just build-jobs "flake: let
        lib = flake.inputs.nixpkgs.lib;
        system = \"{{ arch() }}-linux\";
        selected = [ $(list "${selected[@]}") ];
        owners = name: lib.filter (c: name == c || lib.hasPrefix \"\${c}-\" name) [ $(list "${components[@]}") ];
        owner = name: lib.foldl' (longest: c: if lib.stringLength c > lib.stringLength longest then c else longest) \"\" (owners name);
        owned = name: lib.elem (owner name) selected;
      in {
        packages = lib.filterAttrs (name: _: lib.hasPrefix \"lys-\" name && owned (lib.removePrefix \"lys-\" name)) flake.outputs.packages.\${system};
        checks = lib.filterAttrs (name: check: owned name && !(check ? driver)) flake.outputs.checks.\${system};
        vms = lib.genAttrs [ $(list "${vms[@]}") ] (name: flake.outputs.checks.\${system}.\${name}.driver);
      }"

# The CI job the Go, further and package jobs wait for, on this machine's
# system: the dev shells of the named components, the only Nix outputs those
# jobs use: `just build-shells lib ui wm`.
[script("bash")]
build-shells +components:
    set -euo pipefail
    list() { for name; do printf '"%s" ' "$name"; done; }
    just build-jobs "flake: let
        lib = flake.inputs.nixpkgs.lib;
        shells = flake.outputs.devShells.{{ arch() }}-linux;
        named = [ $(list {{ components }}) ];
        missing = lib.filter (name: !(shells ? \${name})) named;
      in
        if missing != [ ] then throw \"build-shells: \${toString missing} have no dev shell on {{ arch() }}-linux\"
        else { devShells = lib.getAttrs named shells; }"

# Build every derivation a nix-eval-jobs selection of the flake picks, in one
# nix build that keeps going past a failure, then realise each selected dev
# shell's environment the way `nix develop` and `just gates` load it.
[private]
[script("bash")]
build-jobs select:
    set -euo pipefail
    dir=$(mktemp -d)
    trap 'rm -rf "$dir"' EXIT
    just eval-jobs {{ quote(select) }} --check-cache-status > "${dir}/jobs"
    if ! jq -e -s 'length > 0' "${dir}/jobs" >/dev/null; then
        echo "build-jobs: the selection picks nothing to build on {{ arch() }}-linux" >&2
        exit 1
    fi
    jq -r 'select(.cacheStatus == "cached") | "build-jobs: \(.attr) is in the binary cache already"' "${dir}/jobs"
    mapfile -t drvs < <(jq -r 'select(.cacheStatus != "cached") | .drvPath + "^*"' "${dir}/jobs")
    if (( ${#drvs[@]} > 0 )); then
        nix build -L --keep-going --no-link "${drvs[@]}"
    fi
    for shell in $(jq -r 'select(.attr | startswith("devShells.")) | .attr | ltrimstr("devShells.")' "${dir}/jobs"); do
        nix print-dev-env --profile "${dir}/${shell}" ".#${shell}" >/dev/null
    done

# Evaluate the derivations a nix-eval-jobs selection of the flake picks, on as
# many workers as cores and memory allow, and print them as JSON lines; any
# evaluation error fails it, naming the attribute.
[private]
[script("bash")]
eval-jobs select *flags:
    set -euo pipefail
    memory=$(awk '/^MemTotal:/ { print int($2 / 1048576) }' /proc/meminfo)
    workers=$(( memory / 4 < $(nproc) ? memory / 4 : $(nproc) ))
    out=$(mktemp)
    trap 'rm -f "$out"' EXIT
    nix run --inputs-from . nixpkgs#nix-eval-jobs -- --flake . --force-recurse --workers "$(( workers > 0 ? workers : 1 ))" --max-memory-size 3072 {{ flags }} --select {{ quote(select) }} > "$out"
    if jq -e -s 'any(.[]; has("error") and .error != null)' "$out" >/dev/null; then
        jq -r 'select(.error != null) | "eval-jobs: \(.attr) does not evaluate:\n\(.error)"' "$out" >&2
        exit 1
    fi
    cat "$out"

# Run the Go gates and race suites of the components (every one when none is
# named) in parallel, each gate recipe in its own process in its component's
# dev shell, sharing one Go build cache, as many at once as cores and memory
# allow, and print the wall clock: `just gates`, `just gates wm shell`. A
# target `<component>:gates` runs only its Go gates and `<component>:race` only
# its race suite, as CI's Go jobs pick them. On a system `gate-runners` in
# ci/components.json names, only the gates it lists there run. A component with `shards` in
# ci/components.json splits its race suite and the gates `sharded-gates` names
# into one part per shard and a part for every other package; `wm/2` runs only
# the second part's packages of those, and the part after the shards, `wm/4`
# for wm's three shards, runs the rest of them and every other gate but the
# ones `static-gates` names, which run in a last part of their own, `wm/5`, so
# they run beside the tests rather than after them: `just gates wm/2:race
# wm:gates`. A shard entry that names tests of a package
# splits that one package: its part runs only those tests, with `go test -run`,
# and the last part runs the package with every test a shard names skipped. A
# split recipe runs once per package it splits, beside the run of its whole
# packages, each a process of its own that reads its packages, -run and -skip
# patterns from $LYS_TEST_PACKAGES, $LYS_TEST_RUN and $LYS_TEST_SKIP. Before
# any of it, each target component's justfile must pass `just --fmt --check`.
[script("bash")]
gates *targets:
    set -euo pipefail
    start=$SECONDS
    read -ra targets <<< {{ quote(targets) }}
    if (( ${#targets[@]} == 0 )); then
        mapfile -t targets < <(jq -r '.components | to_entries[] | select(.value.gates or .value.race) | .key' ci/components.json)
    fi
    tasks=()
    compile=0
    declare -A pkgs=() runs=() skips=() rest=() selects=() dropped=() seen=() splits=([race]=1)
    arch={{ quote(arch()) }}
    while read -r gate; do splits[$gate]=1; done < <(jq -r '.["sharded-gates"][]' ci/components.json)
    # shellcheck disable=SC2016
    units='
        .components[$c].shards as $shards
        | if $part <= ($shards | length) then
            $shards[$part - 1]
            | ([.[] | strings] | select(length > 0) | "packages - " + join(" ")),
              (.[] | objects | "run ^(\(.tests | join("|")))$ " + .package)
          else
            ([$shards[][] | if type == "string" then . else .package end] | unique | "rest - " + join(" ")),
            ([$shards[][] | objects] | group_by(.package)[] | "skip ^(\([.[].tests[]] | join("|")))$ " + .[0].package)
          end'
    for spec in "${targets[@]}"; do
        target=${spec%%:*}
        suite=${spec#"$target"}
        suite=${suite#:}
        case "$suite" in
            "") kinds=(gates race) ;;
            gates | race) kinds=("$suite") ;;
            *)
                echo "gates: ${spec} names the suite ${suite}, which is neither gates nor race" >&2
                exit 1
                ;;
        esac
        c=${target%%/*}
        part=${target#"$c"}
        part=${part#/}
        if ! jq -e --arg c "$c" '.components | has($c)' ci/components.json >/dev/null; then
            echo "gates: ${c} is no component of ci/components.json" >&2
            exit 1
        fi
        read -r shards parts < <(jq -r --arg c "$c" --arg arch "$arch" '
            .["gate-runners"][$arch] as $only
            | (.["static-gates"] // []) as $static
            | .components[$c] | (.shards // [] | length) as $n
            | [$n, if $n == 0 then 0 elif any((.gates // [])[]; . as $g | ($only == null or ($only | index($g))) and ($static | index($g))) then $n + 2 else $n + 1 end]
            | @tsv
        ' ci/components.json)
        if [[ "$target" == */* ]] && ! (( shards > 0 )); then
            echo "gates: ${target} is no part of ${c}, which has no shards" >&2
            exit 1
        fi
        if [[ "$target" == */* ]] && ! [[ "$part" =~ ^[1-9][0-9]*$ && "$part" -le "$parts" ]]; then
            echo "gates: ${target} is no part of ${c}, which has parts 1 to ${parts} on ${arch}" >&2
            exit 1
        fi
        count=${#tasks[@]}
        for kind in "${kinds[@]}"; do
            while read -r recipe; do
                task="${target} ${recipe}"
                if [[ -v seen[$task] ]]; then
                    echo "gates: ${spec} runs ${task} a second time" >&2
                    exit 1
                fi
                seen[$task]=1
                if [[ "$kind" == gates ]]; then compile=1; fi
                if [[ ! -v splits[$recipe] ]] || [[ -z "$part" ]] || (( shards == 0 )); then
                    tasks+=("$task")
                    continue
                fi
                while read -r scope pattern list; do
                    case "$scope" in
                        packages) unit=$task; pkgs[$unit]=$list ;;
                        rest) unit=$task; rest[$unit]=$list ;;
                        run) unit="${task} ${list}"; pkgs[$unit]=$list; runs[$unit]=$pattern ;;
                        skip) unit="${task} ${list}"; pkgs[$unit]=$list; skips[$unit]=$pattern; selects[$task]=1 ;;
                    esac
                    tasks+=("$unit")
                done < <(jq -r --arg c "$c" --argjson part "$part" "$units" ci/components.json)
            done < <(jq -r --arg c "$c" --arg kind "$kind" --arg arch "$arch" --argjson part "${part:-0}" '
                .["gate-runners"][$arch] as $only
                | .["sharded-gates"] as $split
                | (.["static-gates"] // []) as $static
                | .components[$c] | (.shards // [] | length) as $n
                | if $kind == "race" then (select(.race) | "race") else (.gates // [])[] | select(. as $g | $only == null or ($only | index($g))) end
                | select(. as $g | ($static | index($g)) as $checks
                    | $part == 0
                    or $part <= $n and ($g == "race" or ($split | index($g)))
                    or $part == $n + 1 and ($checks | not)
                    or $part == $n + 2 and $checks)
            ' ci/components.json)
        done
        if (( ${#tasks[@]} == count )); then
            echo "gates: ${spec} runs nothing, since ${c} has no ${kinds[*]} to run" >&2
            exit 1
        fi
    done
    dir=$(mktemp -d)
    trap 'jobs -pr | xargs -r kill 2>/dev/null; wait; rm -rf "$dir"' EXIT
    declare -A running=() started=() envs=()
    mapfile -t components < <(printf '%s\n' "${tasks[@]}" | cut -d' ' -f1 | cut -d/ -f1 | sort -u)
    for c in "${components[@]}"; do
        if ! just --fmt --check --justfile "${c}/justfile" > "${dir}/fmt.log" 2>&1; then
            echo "gates: ${c}/justfile is not formatted; just --fmt --justfile ${c}/justfile fixes it:" >&2
            cat "${dir}/fmt.log" >&2
            exit 1
        fi
    done
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
            case "$recipe" in
                race) export CGO_ENABLED=1; build=(-race) ;;
                test-lysdebug) build=(-tags lysdebug) ;;
            esac
            go list "${build[@]}" ./... | sort > "${dir}/all"
            go list "${build[@]}" "${sharded[@]}" | sort > "${dir}/sharded"
        ) 2> "${dir}/list.log"; then
            echo "gates: the packages of ${target} ${recipe} outside ${c}'s shards do not list:" >&2
            cat "${dir}/list.log" >&2
            exit 1
        fi
        pkgs[$task]=$(comm -23 "${dir}/all" "${dir}/sharded" | paste -sd' ')
        if [[ -n "${pkgs[$task]}" ]]; then continue; fi
        if [[ ! -v selects[$task] ]]; then
            echo "gates: ${c}'s shards hold every package, so ${target} ${recipe} would test none" >&2
            exit 1
        fi
        dropped[$task]=1
    done
    kept=()
    for task in "${tasks[@]}"; do
        if [[ ! -v dropped[$task] ]]; then kept+=("$task"); fi
    done
    tasks=("${kept[@]}")
    if (( compile )) && (( ${#tasks[@]} > 1 )); then
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
            printf 'ok    %-40s %4ss\n' "$task" "$((SECONDS - started[$pid]))"
        else
            printf 'FAIL  %-40s %4ss  exit %s\n' "$task" "$((SECONDS - started[$pid]))" "$status"
            failed+=("$task")
        fi
    }
    for task in "${tasks[@]}"; do
        while (( ${#running[@]} >= slots )); do reap; done
        read -r target recipe _ <<< "$task"
        c=${target%%/*}
        (
            set +euo pipefail
            # shellcheck source=/dev/null
            source "${envs[$c]}"
            unset LYS_TEST_PACKAGES LYS_TEST_RUN LYS_TEST_SKIP
            if [[ -v pkgs[$task] ]]; then export LYS_TEST_PACKAGES="${pkgs[$task]}"; fi
            if [[ -v runs[$task] ]]; then export LYS_TEST_RUN="${runs[$task]}"; fi
            if [[ -v skips[$task] ]]; then export LYS_TEST_SKIP="${skips[$task]}"; fi
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
        [$components[0].components[] | .vms[]?[]] as $ci
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

# Write nix/go-packages.json: each Go package's imports within the module and
# from outside it, and its embedded files, which the packages' filtered sources
# and Go build caches in nix/go.nix follow.
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
