#!/usr/bin/env bash
# Mutation testing with gremlins — the honest test-quality signal that line
# coverage cannot give: it breaks the code on purpose and checks whether the
# test suite notices.
#
#   scripts/mutate.sh                 # run the logical packages (lexique, engine)
#   scripts/mutate.sh ./engine        # run a single package
#   MSI_MIN=80 scripts/mutate.sh      # fail if the mutation score drops below 80%
#
# Deliberately NOT part of `mise run ci`: a full run is slow (tens of seconds
# per package, worse on a full module) and gating every commit on a mutation
# score invites padding tests to move a number rather than to catch bugs.
# Run it by hand, or on a schedule, and read the survivors — that is where the
# signal is.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

# Packages that carry real logic (spelling analysis, game rules) rather than
# glue — the ones worth the time mutation testing costs.
default_pkgs=(./lexique ./engine)
pkgs=("${@:-${default_pkgs[@]}}")

# gremlins' own coverage-gathering step is fast, but running every test binary
# once per surviving mutant with the default worker/timeout settings thrashes
# on a busy machine and times mutants out instead of killing or sparing them.
# One worker, two test CPUs and a generous timeout coefficient trade wall time
# for a result that means something.
workers="${MUTATE_WORKERS:-1}"
test_cpu="${MUTATE_TEST_CPU:-2}"
timeout_coefficient="${MUTATE_TIMEOUT_COEFFICIENT:-20}"
min="${MSI_MIN:-0}"

out_dir="$(mktemp -d)"
trap 'rm -rf "$out_dir"' EXIT

worst=100
for pkg in "${pkgs[@]}"; do
  name="$(basename "$pkg")"
  report="$out_dir/$name.json"
  printf '\033[34m→ mutating %s\033[0m\n' "$pkg"
  gremlins unleash "$pkg" \
    --workers "$workers" \
    --test-cpu "$test_cpu" \
    --timeout-coefficient "$timeout_coefficient" \
    -o "$report"

  killed="$(jq '.mutants_killed' "$report")"
  lived="$(jq '.mutants_lived' "$report")"
  not_covered="$(jq '.mutants_not_covered' "$report")"
  msi="$(awk -v k="$killed" -v l="$lived" 'BEGIN{t=k+l; printf "%.2f", (t>0 ? 100*k/t : 100)}')"
  worst="$(awk -v w="$worst" -v m="$msi" 'BEGIN{print (m+0<w+0)?m:w}')"

  printf '  killed=%s lived=%s not-covered=%s MSI=%s%%\n' "$killed" "$lived" "$not_covered" "$msi"
done

printf '\033[34mWorst MSI across the run: %s%%\033[0m\n' "$worst"
if awk -v w="$worst" -v m="$min" 'BEGIN{exit !(w+0 < m+0)}'; then
  printf '\033[31m✗ MSI %s%% is below the required %s%%.\033[0m\n' "$worst" "$min" >&2
  exit 1
fi
printf '\033[32m✓ MSI %s%% meets the %s%% minimum.\033[0m\n' "$worst" "$min"
