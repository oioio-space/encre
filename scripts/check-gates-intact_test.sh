#!/usr/bin/env bash
# Bite test for scripts/check-gates-intact.sh.
#
# A guardrail-inventory check that is itself unverified is one more thing held
# by convention — the defect it exists to catch, applied to itself. So this
# asserts BOTH directions for every category that carries weight: that removing
# a guardrail fails, and that a matching configuration passes.
#
# Each case runs against a throwaway git repo built from scratch in $TMPDIR, so
# nothing here can touch the real working tree.
#
# Run: mise run check-gates-intact:test
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
script="$repo_root/scripts/check-gates-intact.sh"

pass=0 fail=0

# fixture DIR : a minimal repo whose live guardrails exactly match its manifest.
fixture() {
  local d="$1"
  mkdir -p "$d"/{.githooks,.beads/hooks,.claude/hooks,scripts}
  git -C "$d" init -q
  git -C "$d" config user.email t@t && git -C "$d" config user.name t

  cat >"$d/.githooks/pre-commit" <<'EOF'
#!/usr/bin/env bash
gate "Tidying…" \
  "blocked" \
  "clean:check" ./scripts/clean.sh
gate "Linting…" \
  "blocked" \
  "lint:staged" ./scripts/lint.sh
marker="claude-simplify-ok"
EOF
  for h in pre-commit pre-push post-commit commit-msg; do
    [[ -f "$d/.githooks/$h" ]] || printf '#!/usr/bin/env bash\ntrue\n' >"$d/.githooks/$h"
    chmod +x "$d/.githooks/$h"
    # shellcheck disable=SC2016  # the $(…) is the shim's body, written literally,
    # not something to expand here — expanding it would bake this test's own path in.
    printf '#!/usr/bin/env bash\nexec "$(git rev-parse --show-toplevel)/.githooks/%s" "$@"\n' "$h" >"$d/.beads/hooks/$h"
    chmod +x "$d/.beads/hooks/$h"
  done

  cat >"$d/.golangci.yml" <<'EOF'
linters:
  enable:
    - govet
    - revive
  settings:
    depguard:
      rules:
        engine:
          list-mode: strict
EOF

  cat >"$d/mise.toml" <<'EOF'
[env]
COVER_MIN = "85"

[tasks.ci]
depends = ["lint", "test:ci"]
EOF

  cat >"$d/.claude/settings.json" <<'EOF'
{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":"$CLAUDE_PROJECT_DIR/.claude/hooks/cycle-check.sh"}]}]}}
EOF
  printf '#!/usr/bin/env bash\ntrue\n' >"$d/.claude/hooks/cycle-check.sh"

  printf '#!/usr/bin/env bash\ngo run ./cmd/glyphcheck ./...\n' >"$d/scripts/lint.sh"

  cat >"$d/scripts/gates.manifest" <<'EOF'
precommit:clean:check
precommit:lint:staged
precommit:simplify-attestation
linter:govet
linter:revive
depguard-rule:engine
ci-dep:lint
ci-dep:test:ci
param:COVER_MIN=85
claude-hook:cycle-check.sh
hook-shim:commit-msg
hook-shim:post-commit
hook-shim:pre-commit
hook-shim:pre-push
githook:commit-msg
githook:post-commit
githook:pre-commit
githook:pre-push
lintsh:glyphcheck
EOF
  git -C "$d" add -A >/dev/null 2>&1
  git -C "$d" commit -qm init --no-verify >/dev/null 2>&1
}

# check WANT LABEL MUTATOR : WANT is "pass" or "fail"; MUTATOR runs with $d set.
check() {
  local want="$1" label="$2" mutate="$3" d out got=pass
  d="$(mktemp -d)"
  fixture "$d"
  ( cd "$d" && eval "$mutate" )
  out="$( cd "$d" && bash "$script" 2>&1 )" || got=fail
  rm -rf "$d"
  if [[ "$got" == "$want" ]]; then
    printf '\033[32m  ✓ %s (%s)\033[0m\n' "$label" "$got"
    pass=$((pass + 1))
  else
    printf '\033[31m  ✗ %s: expected %s, got %s\033[0m\n' "$label" "$want" "$got" >&2
    printf '%s\n' "$out" | sed 's/^/      /' >&2
    fail=$((fail + 1))
  fi
}

echo "▶ scripts/check-gates-intact.sh"
check pass "untouched fixture matches its manifest"   "true"
check fail "a linter removed from .golangci.yml"      "sed -i '/- revive/d' .golangci.yml"
check fail "a task removed from [tasks.ci] depends"   "sed -i 's/, \"test:ci\"//' mise.toml"
check fail "a pre-commit gate removed"                "sed -i '/lint:staged/d' .githooks/pre-commit"
check fail "the /simplify attestation gate removed"   "sed -i '/claude-simplify-ok/d' .githooks/pre-commit"
check fail "COVER_MIN gutted in place (85 -> 10)"     "sed -i 's/COVER_MIN = \"85\"/COVER_MIN = \"10\"/' mise.toml"
check fail "a depguard import-boundary rule removed"  "sed -i '/engine:/,+1d' .golangci.yml"
check fail "a Claude hook unwired from settings.json" "printf '%s' '{\"hooks\":{}}' > .claude/settings.json"
check fail "a shim turned back into a verbatim copy"  "printf '#!/usr/bin/env bash\ntrue\n' > .beads/hooks/pre-commit"
check fail "a versioned githook made non-executable"  "chmod -x .githooks/commit-msg"
check fail "a lint.sh cmd-linter invocation deleted"  "sed -i '/go run/d' scripts/lint.sh"
check pass "an ADDED guardrail only warns"            "sed -i 's/    - revive/    - revive\n    - godot/' .golangci.yml"

printf '%s\n' "──────────"
if [[ $fail -gt 0 ]]; then
  printf '\033[31m✗ %d passed, %d FAILED\033[0m\n' "$pass" "$fail" >&2
  exit 1
fi
printf '\033[32m✓ %d passed\033[0m\n' "$pass"
