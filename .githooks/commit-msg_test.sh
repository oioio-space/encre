#!/usr/bin/env bash
# Bite test for .githooks/commit-msg — the gate that forces every commit to cite
# its bead. A gate nobody exercises is a gate you discover is broken on the day
# it was supposed to stop you, so this asserts both halves: what it must refuse
# AND what it must let through.
#
# Run: mise run commit-msg:test
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

hook=".githooks/commit-msg"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

pass=0 fail=0

# check WANT LABEL MESSAGE : WANT is "accept" or "refuse".
check() {
  local want="$1" label="$2" message="$3" got
  printf '%s\n' "$message" >"$tmp/msg"
  if "$hook" "$tmp/msg" >/dev/null 2>&1; then got=accept; else got=refuse; fi
  if [[ "$got" == "$want" ]]; then
    printf '\033[32m  ✓ %s (%s)\033[0m\n' "$label" "$got"
    pass=$((pass + 1))
  else
    printf '\033[31m  ✗ %s: expected %s, got %s\033[0m\n' "$label" "$want" "$got" >&2
    fail=$((fail + 1))
  fi
}

# A real bead id to assert the happy path against; without one the accept cases
# would pass for the wrong reason (no database, no verification).
existing="$(bd list --json 2>/dev/null | python3 -c 'import json,sys
d = json.load(sys.stdin)
issues = d if isinstance(d, list) else d.get("issues", [])
print(issues[0]["id"] if issues else "")' 2>/dev/null || true)"
if [[ -z "$existing" ]]; then
  # The Dolt database is gitignored and bd is not a CI tool, so a fresh clone has
  # no beads to verify ids against. Skip LOUDLY rather than fail — but never
  # silently: a test that quietly no-ops reads as coverage it does not provide.
  # This test earns its keep locally, which is where the commit-msg gate runs.
  echo "⊘ commit-msg_test: SKIPPED — no bead database reachable (expected in CI;"
  echo "  run it locally, where the gate it covers actually fires)."
  exit 0
fi

echo "▶ .githooks/commit-msg (against existing bead $existing)"
check refuse "no Bead: trailer"            "feat: something"
check refuse "bead named only in prose"    "$(printf 'feat: x\n\nfixes %s' "$existing")"
check refuse "malformed id"                "$(printf 'feat: x\n\nBead: encre-WAYTOOLONG')"
check refuse "id absent from bd"           "$(printf 'feat: x\n\nBead: encre-zzz')"
check refuse "4-char id absent from bd"    "$(printf 'feat: x\n\nBead: encre-zzzz')"
check refuse "foreign prefix"              "$(printf 'feat: x\n\nBead: fk-abc')"
check accept "valid id"                    "$(printf 'feat: x\n\nBead: %s' "$existing")"
check accept "two ids, comma separated"    "$(printf 'feat: x\n\nBead: %s, %s' "$existing" "$existing")"
check accept "merge commit is exempt"      "Merge branch 'x'"
check accept "fixup! is exempt"            "fixup! feat: x"
check accept "squash! is exempt"           "squash! feat: x"

printf '%s\n' "──────────"
if [[ $fail -gt 0 ]]; then
  printf '\033[31m✗ %d passed, %d FAILED\033[0m\n' "$pass" "$fail" >&2
  exit 1
fi
printf '\033[32m✓ %d passed\033[0m\n' "$pass"
