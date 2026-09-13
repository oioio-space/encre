#!/usr/bin/env bash
# Guardrail-inventory gate: fails if the set of ACTIVE guardrails (pre-commit
# gates, enabled linters, depguard import-boundary rules, `ci`'s task
# dependencies, force-bearing parameters, wired Claude review hooks, the
# versioned git hooks and the beads shims that must delegate to them) has shrunk
# relative to the committed manifest (scripts/gates.manifest), without that
# shrink being made EXPLICIT in the manifest itself.
#
# WHY THIS EXISTS: this project's whole gate chain is held by files that other
# tools happily rewrite. The proof is local and recent — on 2026-09-13 `bd init`
# took core.hooksPath and wrote verbatim COPIES of .githooks/{pre-commit,
# pre-push,post-commit} into .beads/hooks/. Nothing was broken that day, so
# nothing complained; the copies would simply have stopped matching the moment
# .githooks changed, and every kit gate would have quietly gone stale. Only
# reading the diff caught it. This script makes that mechanical: it reads the
# live configuration every run and compares it to a committed inventory.
#
# HOW THIS SCRIPT IS INVOKED — never ONLY through mise.toml's `[tasks.ci]
# depends` list, because that list is itself one of the things it watches: if
# invocation depended solely on being named there, deleting that one string
# would disable the whole check with nothing left to notice. It is therefore
# also called by path from two anchors that do not go through `depends`:
# .github/workflows/ci.yml runs it as its own step, and .githooks/pre-push calls
# it directly. Removing it from `[tasks.ci] depends` still gets caught — the
# anchors keep running it, and its own ci-dep:* comparison reports the missing
# `ci-dep:check-gates-intact` line.
#
# WHAT COUNTS AS A GUARDRAIL, and how each is read from the live source:
#   precommit:<mise-task>  — each `gate "<header>" "<fail>" "<task>" …` call in
#                            .githooks/pre-commit, keyed by mise task name
#                            (stable; header prose is not), plus the sentinel
#                            `simplify-attestation` for the one gate with no
#                            mise task (the /simplify marker check).
#   linter:<name>          — entries under `linters.enable:` in .golangci.yml.
#                            `formatters.enable` (gofumpt) is deliberately not
#                            tracked: formatters are not lint checks.
#   depguard-rule:<name>   — top-level rule names under
#                            linters.settings.depguard.rules. None exist yet;
#                            the extractor stays so that adding an
#                            import-boundary rule later is noticed (as an
#                            "active but unmanifested" warning) instead of
#                            being born unwatched.
#   ci-dep:<task>          — task names in mise.toml's `[tasks.ci] depends`.
#   param:<name>=<value>   — force-bearing settings, checked by VALUE, not name:
#                            a gate whose name survives while its strength is
#                            gutted in place (COVER_MIN 85 -> 10) passes every
#                            name-based check above. Tracks mise.toml's `[env]
#                            COVER_MIN`. Deliberately NOT covered, scope kept
#                            narrow: `|| true` added inside a gate script, gosec
#                            #nosec additions, and any other force-bearing value
#                            not listed here.
#   claude-hook:<file>     — each .claude/hooks/*.sh referenced by a `command`
#                            in .claude/settings.json. A hook file left on disk
#                            but no longer referenced — unwired, not deleted —
#                            counts as missing.
#   hook-shim:<name>       — bd owns core.hooksPath, so the file that actually
#                            runs on commit/push may be .beads/hooks/<name>. For
#                            pre-commit/pre-push/post-commit/commit-msg, a shim
#                            counts as live ONLY if it contains the literal
#                            ".githooks/<name>", i.e. it delegates rather than
#                            re-implements. This is the category that would have
#                            caught the 2026-09-13 copies.
#   githook:<name>         — same names; live if .githooks/<name> exists and is
#                            executable — the versioned gate file itself,
#                            independent of which hooksPath a clone has wired.
#   lintsh:<name>          — each `go run ./cmd/<name>` invocation in
#                            scripts/lint.sh. None exist yet; kept for the same
#                            reason as depguard-rule.
#
# MANIFEST FORMAT: one `category:name` token per line in scripts/gates.manifest,
# blank lines and #-comments ignored. Order is not meaningful, only membership.
#
# BEHAVIOR:
#   - in manifest, missing live  -> FAIL, naming the guardrail and its source.
#     Retiring a guardrail on purpose stays possible: delete its manifest line
#     in the SAME commit, which makes the removal explicit and reviewable.
#   - live, not in manifest      -> WARN (exit 0). Adding a gate must never be
#     blocked by this script.
#   - exact match                -> pass.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

precommit_file=".githooks/pre-commit"
golangci_file=".golangci.yml"
mise_file="mise.toml"
settings_file=".claude/settings.json"
beads_hooks_dir=".beads/hooks"
lint_file="scripts/lint.sh"
manifest="scripts/gates.manifest"

if [[ ! -f "$manifest" ]]; then
  echo "✗ check-gates-intact: $manifest not found — nothing to compare against." >&2
  exit 1
fi

# ---- Extract the LIVE guardrail set -----------------------------------------

live=()

# precommit:<mise-task> — the 3rd argument of each `gate "…" "…" "task" …` call
# sits alone on a 2-space-indented continuation line and is lowercase and
# space-free; header and fail-message strings are capitalized prose containing
# spaces, so this pattern cannot match them by accident.
while IFS= read -r task; do
  live+=("precommit:$task")
done < <(grep -oE '^  "[a-z][a-z0-9:._-]*"' "$precommit_file" | tr -d '" ')

# The /simplify attestation gate has no mise task — it reads a marker file
# directly, so its presence is keyed on that fixed filename.
if grep -q 'claude-simplify-ok' "$precommit_file"; then
  live+=("precommit:simplify-attestation")
fi

# linter:<name> — bounded to the block between the top-level `linters:` key and
# its `enable:` child, ending at the next key at 2-space indent (`settings:`).
while IFS= read -r name; do
  live+=("linter:$name")
done < <(awk '
  /^linters:$/ { in_linters=1; next }
  /^[a-zA-Z]/ { in_linters=0 }
  in_linters && /^  enable:$/ { in_enable=1; next }
  in_linters && in_enable {
    if ($0 ~ /^    - [a-zA-Z0-9_-]+/) {
      line=$0
      sub(/^    - /, "", line)
      sub(/[[:space:]]*#.*$/, "", line)
      print line
      next
    }
    in_enable=0
  }
' "$golangci_file")

# depguard-rule:<name> — 8-space-indented keys under the depguard rules block.
while IFS= read -r name; do
  live+=("depguard-rule:$name")
done < <(awk '
  /^    depguard:$/ { in_depguard=1; next }
  in_depguard && /^      rules:$/ { in_rules=1; next }
  in_depguard && in_rules {
    if ($0 ~ /^        [A-Za-z0-9_]+:$/) {
      line=$0
      sub(/^ +/, "", line)
      sub(/:$/, "", line)
      print line
      next
    }
    if ($0 ~ /^    [A-Za-z]/) { in_depguard=0; in_rules=0 }
  }
' "$golangci_file")

# ci-dep:<task> — mise renders the list as one `depends = ["a", "b", …]` line.
ci_depends_line="$(awk '/^\[tasks\.ci\]/{f=1} f && /^depends = \[/{print; exit}' "$mise_file")"
while IFS= read -r task; do
  [[ -n "$task" ]] && live+=("ci-dep:$task")
done < <(printf '%s\n' "$ci_depends_line" | grep -oE '"[^"]+"' | tr -d '"')

# param:<name>=<value> — bounded to the [env] table, ending at the next header.
cover_min="$(awk '
  /^\[env\]/ { f=1; next }
  /^\[/ { f=0 }
  f && /^COVER_MIN[[:space:]]*=/ {
    line=$0
    sub(/^COVER_MIN[[:space:]]*=[[:space:]]*"/, "", line)
    sub(/".*$/, "", line)
    print line
    exit
  }
' "$mise_file")"
if [[ -n "$cover_min" ]]; then
  live+=("param:COVER_MIN=$cover_min")
fi

# claude-hook:<file> — every .claude/hooks/*.sh named by a `command` value.
if [[ -f "$settings_file" ]]; then
  while IFS= read -r hook; do
    [[ -n "$hook" ]] && live+=("claude-hook:$hook")
  done < <(grep -oE '\.claude/hooks/[A-Za-z0-9_-]+\.sh' "$settings_file" | xargs -n1 basename | sort -u)
fi

# hook-shim:<name> — a .beads/hooks/<name> that exists but does NOT delegate is
# deliberately left OUT of `live`, so a re-implemented copy is reported exactly
# like a deleted gate. githook:<name> — the versioned file itself.
for name in pre-commit pre-push post-commit commit-msg; do
  shim="$beads_hooks_dir/$name"
  if [[ -f "$shim" ]] && grep -qF ".githooks/$name" "$shim"; then
    live+=("hook-shim:$name")
  fi

  githook="$(dirname "$precommit_file")/$name"
  if [[ -x "$githook" ]]; then
    live+=("githook:$name")
  fi
done

# lintsh:<name> — each `go run ./cmd/<name>` still invoked by scripts/lint.sh.
if [[ -f "$lint_file" ]]; then
  while IFS= read -r name; do
    [[ -n "$name" ]] && live+=("lintsh:$name")
  done < <(grep -oE 'go run \./cmd/[A-Za-z0-9_-]+' "$lint_file" | sed -E 's#go run \./cmd/##' | sort -u)
fi

# Non-blocking local-config warning: core.hooksPath pointing anywhere other than
# .githooks or .beads/hooks routes around every gate above, and is per-clone git
# config that CI can never see. It cannot be a hard failure — a fresh clone that
# has not run `mise run setup` yet has no hooksPath at all — so it only warns.
hooks_path="$(git config core.hooksPath 2>/dev/null || true)"
if [[ -n "$hooks_path" ]]; then
  # bd stores an absolute path; resolve both sides before comparing so
  # `.beads/hooks` and `$repo_root/.beads/hooks` are the same target.
  hooks_path_abs="$(cd "$hooks_path" 2>/dev/null && pwd || true)"
  githooks_abs="$(cd .githooks 2>/dev/null && pwd || true)"
  beads_hooks_abs="$(cd "$beads_hooks_dir" 2>/dev/null && pwd || true)"
  if [[ "$hooks_path_abs" != "$githooks_abs" && "$hooks_path_abs" != "$beads_hooks_abs" ]]; then
    echo "⚠ check-gates-intact: git config core.hooksPath is '$hooks_path', neither .githooks nor $beads_hooks_dir — the versioned gates may not be running (local config only, invisible to CI)." >&2
  fi
fi

# ---- Compare against the manifest -------------------------------------------

mapfile -t manifest_entries < <(grep -vE '^[[:space:]]*(#|$)' "$manifest")

missing=()
for entry in "${manifest_entries[@]}"; do
  found=0
  for l in "${live[@]}"; do
    if [[ "$l" == "$entry" ]]; then
      found=1
      break
    fi
  done
  if [[ $found -eq 0 ]]; then
    missing+=("$entry")
  fi
done

extra=()
for l in "${live[@]}"; do
  found=0
  for entry in "${manifest_entries[@]}"; do
    if [[ "$l" == "$entry" ]]; then
      found=1
      break
    fi
  done
  if [[ $found -eq 0 ]]; then
    extra+=("$l")
  fi
done

status=0

if [[ ${#missing[@]} -gt 0 ]]; then
  echo "✗ check-gates-intact: guardrail(s) present in $manifest but missing from the live configuration:" >&2
  for entry in "${missing[@]}"; do
    case "$entry" in
    precommit:*) src="$precommit_file (gate call, or the /simplify attestation marker check)" ;;
    linter:*) src="$golangci_file (linters.enable)" ;;
    depguard-rule:*) src="$golangci_file (linters.settings.depguard.rules)" ;;
    ci-dep:*) src="$mise_file ([tasks.ci] depends)" ;;
    param:*) src="$mise_file ([env] block)" ;;
    claude-hook:*) src="$settings_file (hook wiring)" ;;
    hook-shim:*) src="$beads_hooks_dir (must delegate to .githooks/<name>)" ;;
    githook:*) src=".githooks/<name> (must exist and be executable)" ;;
    lintsh:*) src="$lint_file (must still run \`go run ./cmd/<name>\`)" ;;
    *) src="$manifest" ;;
    esac
    echo "    - $entry  (declared in $src)" >&2
  done
  echo "  If this guardrail was DELIBERATELY removed or renamed, update $manifest in the" >&2
  echo "  SAME commit so the removal is explicit and reviewable, then re-run this check." >&2
  status=1
fi

if [[ ${#extra[@]} -gt 0 ]]; then
  echo "⚠ check-gates-intact: guardrail(s) active but not yet in $manifest (not blocking):" >&2
  for entry in "${extra[@]}"; do
    echo "    + $entry" >&2
  done
  echo "  Add these lines to $manifest so future removals of them are caught." >&2
fi

if [[ $status -eq 0 ]]; then
  echo "✓ check-gates-intact: all ${#manifest_entries[@]} manifested guardrails are active."
fi

exit "$status"
