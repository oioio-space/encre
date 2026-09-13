#!/usr/bin/env bash
# Bite test for .claude/hooks/cycle-check.sh.
#
# The hook is silent by design, which makes it the easiest kind of guardrail to
# break without noticing: a typo that makes every signal evaluate to 0 looks
# exactly like a healthy repository. So each signal is asserted in BOTH
# directions — it fires on the state it exists to catch, and it stays quiet on
# the neighbouring state that must not trip it.
#
# Every case runs in a throwaway git repo under $TMPDIR, against a stub `bd` on
# PATH that serves fixture JSON. Nothing here touches the real repo or the real
# bead database.
#
# Run: mise run cycle:check:test
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
hook="$repo_root/.claude/hooks/cycle-check.sh"

command -v jq >/dev/null 2>&1 || { echo "✗ cannot run: jq is required by the hook" >&2; exit 1; }

pass=0 fail=0
stub_dir="$(mktemp -d)"
trap 'rm -rf "$stub_dir"' EXIT

# Stub bd: dispatches on the --status/--all flag and prints the matching fixture
# from $BD_FIXTURES, so a case only has to write the JSON it cares about.
cat >"$stub_dir/bd" <<'STUB'
#!/usr/bin/env bash
f="${BD_FIXTURES:-}"
case "$*" in
*"--status open"*) cat "$f/open.json" 2>/dev/null || echo '[]' ;;
*"--status closed"*) cat "$f/closed.json" 2>/dev/null || echo '[]' ;;
*"--all"*) cat "$f/all.json" 2>/dev/null || echo '[]' ;;
*) echo '[]' ;;
esac
STUB
chmod +x "$stub_dir/bd"

# newrepo : a committed git repo with a PROGRESS.md, and empty bead fixtures.
newrepo() {
  local d
  d="$(mktemp -d)"
  git -C "$d" init -q
  git -C "$d" config user.email t@t
  git -C "$d" config user.name t
  mkdir -p "$d/.fixtures"
  for f in open closed all; do echo '[]' >"$d/.fixtures/$f.json"; done
  echo "# progress" >"$d/PROGRESS.md"
  git -C "$d" add PROGRESS.md
  git -C "$d" commit -qm "docs: init" --no-verify
  printf '%s' "$d"
}

# run DIR : the hook's rendered advice, or "" when it stays silent.
run() {
  ( cd "$1" \
    && PATH="$stub_dir:$PATH" BD_FIXTURES="$1/.fixtures" ENCRE_CYCLE_NOCACHE=1 \
       CLAUDE_PROJECT_DIR="$1" bash "$hook" </dev/null ) \
    | jq -r '.hookSpecificOutput.additionalContext' 2>/dev/null || true
}

# check WANT LABEL SETUP : WANT is a signal name that must appear, or "" for silence.
check() {
  local want="$1" label="$2" setup="$3" d out
  d="$(newrepo)"
  ( cd "$d" && eval "$setup" )
  out="$(run "$d")"
  rm -rf "$d"

  local ok=0
  if [[ -z "$want" ]]; then
    [[ -z "$out" ]] && ok=1
  else
    [[ "$out" == *"$want"* ]] && ok=1
  fi

  if [[ $ok -eq 1 ]]; then
    printf '\033[32m  ✓ %s\033[0m\n' "$label"
    pass=$((pass + 1))
  else
    printf '\033[31m  ✗ %s: expected %s\033[0m\n' "$label" "${want:-silence}" >&2
    printf '%s\n' "${out:-(silence)}" | sed 's/^/      /' >&2
    fail=$((fail + 1))
  fi
}

# epics N : N open P0 epics as the open-bead fixture.
epics() { python3 -c "
import json,sys
n=int(sys.argv[1])
print(json.dumps([{'id':f'encre-e{i:02d}','issue_type':'epic','priority':0,'parent':'x'} for i in range(n)]))
" "$1" >.fixtures/open.json; }

echo "▶ .claude/hooks/cycle-check.sh"

check ""           "healthy repo stays silent"                    "true"

check "ÉVENTAIL"   "7 open P0/P1 epics trips the fan-out"         "epics 7"
check ""           "6 open P0/P1 epics is still under threshold"  "epics 6"

check "HIÉRARCHIE" "an open non-epic bead with no parent"         "echo '[{\"id\":\"encre-abc\",\"issue_type\":\"task\",\"priority\":2,\"parent\":\"\"}]' > .fixtures/open.json"
check ""           "the same bead WITH a parent is fine"          "echo '[{\"id\":\"encre-abc\",\"issue_type\":\"task\",\"priority\":2,\"parent\":\"encre-epi\"}]' > .fixtures/open.json"

promise='[{"id":"encre-pro","status":"closed","title":"t","close_reason":"on surveillera"}]'
check "BOUCLE"     "a bead closed on a promise, no follow-up"     "echo '$promise' > .fixtures/closed.json; echo '$promise' > .fixtures/all.json"
check ""           "same promise, but a follow-up bead cites it"  "echo '$promise' > .fixtures/closed.json; echo '[{\"id\":\"encre-pro\",\"status\":\"closed\",\"close_reason\":\"on surveillera\"},{\"id\":\"encre-fol\",\"status\":\"open\",\"description\":\"suite de encre-pro\"}]' > .fixtures/all.json"
check ""           "a promise in the DESCRIPTION is not a promise" "echo '[{\"id\":\"encre-pro\",\"status\":\"closed\",\"description\":\"corrige ce qui etait tenu par convention\"}]' > .fixtures/closed.json; cp .fixtures/closed.json .fixtures/all.json"

# NON RÉCLAMÉ: cited by a trailer, still open. Created long before the commit ->
# delivered-and-left-open (fires). Created just now -> opened BY that commit
# (silent) — the window exception.
old_bead='[{"id":"encre-old","status":"open","created_at":"2020-01-01T00:00:00Z"}]'
check "NON RÉCLAMÉ" "bead cited by a commit but left open"        "echo '$old_bead' > .fixtures/all.json; git commit -q --allow-empty --no-verify -m 'feat: x

Bead: encre-old'"
check ""            "a bead OPENED by the commit citing it"       "python3 -c \"import json,datetime;print(json.dumps([{'id':'encre-old','status':'open','created_at':datetime.datetime.now(datetime.timezone.utc).isoformat()}]))\" > .fixtures/all.json; git commit -q --allow-empty --no-verify -m 'feat: x

Bead: encre-old'"
check ""            "a cited bead that was closed"                "echo '[{\"id\":\"encre-old\",\"status\":\"closed\",\"created_at\":\"2020-01-01T00:00:00Z\"}]' > .fixtures/all.json; git commit -q --allow-empty --no-verify -m 'feat: x

Bead: encre-old'"

# These commits touch cmd/ on purpose: empty commits count as "process" and
# would trip PROCESSUS too, so the case would pass without isolating DÉRIVE —
# which is exactly what the first draft of this test did.
check "DÉRIVE DU RÉCIT" "13 commits since PROGRESS.md changed"    "mkdir -p cmd/client; for i in \$(seq 13); do echo \$i > cmd/client/f\$i.go; git add -A; git commit -q --no-verify -m \"feat: \$i\"; done"
check ""                "12 commits is still under threshold"     "mkdir -p cmd/client; for i in \$(seq 12); do echo \$i > cmd/client/f\$i.go; git add -A; git commit -q --no-verify -m \"feat: \$i\"; done"

check "PROCESSUS"  "11 of the last 20 commits touch no product"   "for i in \$(seq 11); do mkdir -p docs; echo \$i > docs/d\$i.md; git add -A; git commit -q --no-verify -m \"docs: \$i\"; done"
check ""           "the same commits under cmd/ are product"      "for i in \$(seq 11); do mkdir -p cmd/client; echo \$i > cmd/client/f\$i.go; git add -A; git commit -q --no-verify -m \"feat: \$i\"; done"

check "ARBRE EN CONFLIT" "conflict markers in a tracked file"     "printf '<<<<<<< HEAD\na\n>>>>>>> other\n' > c.txt; git add c.txt; git commit -q --no-verify -m 'x'"
check ""                 "same markers during a real merge"       "printf '<<<<<<< HEAD\na\n>>>>>>> other\n' > c.txt; git add c.txt; git commit -q --no-verify -m 'x'; git rev-parse HEAD > .git/MERGE_HEAD"

printf '%s\n' "──────────"
if [[ $fail -gt 0 ]]; then
  printf '\033[31m✗ %d passed, %d FAILED\033[0m\n' "$pass" "$fail" >&2
  exit 1
fi
printf '\033[32m✓ %d passed\033[0m\n' "$pass"
