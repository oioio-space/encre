#!/usr/bin/env bash
# Claude Code PreToolUse hook (docs-sync layer).
#
# On `git commit`, when the staged change is substantive (touches library/CLI Go
# code, not just tests), this nudges Claude to keep the human-facing record in
# step with the evolution BEFORE the commit lands: README.md + PROGRESS.md should
# reflect what this step changed (features/API/state).
#
# Silent for non-commits and for commits that already stage these (or that touch
# no library/CLI code). The deterministic `.githooks/post-commit` still re-arms
# the /simplify marker; this layer covers only the README/PROGRESS narrative.
set -euo pipefail
# shellcheck source=lib/hooklib.sh
source "$(dirname "$0")/lib/hooklib.sh"

hook_is_git_commit "$(hook_cmd)" || exit 0

cd "$HOOK_PROJECT_DIR" 2>/dev/null || exit 0
staged="$(git diff --cached --name-only 2>/dev/null || true)"
[[ -n "$staged" ]] || exit 0

# Substantive = staged non-test Go under the library/CLI surface: a root package
# file, internal/*.go, or cmd/*.go. Case patterns match `*` against `/` too, so the
# */*  arm plus the nested internal/*.go | cmd/*.go check covers any depth, and the
# bare *.go arm (no `/` in the name) catches the root package file whatever it's named.
code_changed=0
while IFS= read -r f; do
  case "$f" in
    *_test.go) continue ;;
    */*)
      case "$f" in
        internal/*.go | cmd/*.go) code_changed=1 ;;
      esac
      ;;
    *.go) code_changed=1 ;;
  esac
done <<< "$staged"
[[ $code_changed -eq 1 ]] || exit 0

grep -qxF 'README.md'   <<< "$staged" && readme_staged=1   || readme_staged=0
grep -qxF 'PROGRESS.md' <<< "$staged" && progress_staged=1 || progress_staged=0

# Everything already in step → nothing to nudge.
[[ $readme_staged -eq 1 && $progress_staged -eq 1 ]] && exit 0

todo=""
[[ $readme_staged   -eq 0 ]] && todo+="
- **README.md** — if this step changes a user-facing capability, flag, or the value story, update it (skill: readme-author). Skip if purely internal."
[[ $progress_staged -eq 0 ]] && todo+="
- **PROGRESS.md** — record what this step changed (check off shipped items, note decisions)."

hook_emit_context "DOCS-SYNC PRE-COMMIT REVIEW (substantive code is staged):

Keep the human-facing record in step with this step's evolution before committing:
$todo

Routing (token economy): the README/PROGRESS narrative → scribe sub-agent (Haiku). A trivial
or purely-internal diff that changes nothing user-facing can be cleared inline — say so and proceed."
