#!/usr/bin/env bash
# Claude Code UserPromptSubmit hook — development-cycle guardrail.
#
# WHY THIS EXISTS
# This project's doctrine, written into CLAUDE.md on 2026-09-13, is "tenu par des
# mécanismes, pas par la vigilance". Everything that enforces it fires on a
# commit — and a commit is the wrong moment to learn that the hierarchy eroded,
# that six beads were delivered and left open, or that the last twenty commits
# were all process. Those are STATE, not events, and nothing measured them.
# This hook does, on the prompt, before work starts.
#
# DESIGN — three deliberate properties, kept from the fk original:
#
#   1. It measures STATE, never events. Every signal reads the repository and
#      the bead graph as they stand right now. Nothing has to remember to
#      increment a counter, so nothing can forget to.
#
#   2. It is SILENT when nothing is due. A hook that speaks on every prompt
#      becomes noise and gets tuned out — which is the same death as not
#      existing. Silence is what keeps its rare output meaningful.
#
#   3. It never blocks and never fails loudly. It runs on every prompt, so any
#      error degrades to silence (exit 0), never to a broken session.
#
# PORTED FROM fk (.claude/hooks/cycle-check.sh), which grew ten signals. Seven
# are here. Three were deliberately left behind because they measure things this
# repository does not have, and a signal that cannot fire is worse than absent —
# it reads as coverage:
#   - ARBITRAGES   — counts consecutive Fable arbitrations without an audit in
#                    ROADMAP.md's `## Journal Fable`. encre has no Fable
#                    coordinator and no such journal.
#   - LANE PÉRIMÉE — watches fk's Lone Wolf acceptance lane against a cache
#                    marker. No equivalent lane exists here yet.
#   - HORS PLAN    — flags P0/P1 beads that fk's ROADMAP.md does not name. encre
#                    has no document that orders work BY BEAD ID: the order lives
#                    in the graph (priorities + blocking edges) and in
#                    brief/ENCRE_05_backlog.md, which predates the beads and
#                    names tickets, not ids. Ported as-is it would fire on all 44
#                    open beads at the first prompt. A backlog-coverage signal
#                    shaped for this repo is filed as its own bead instead of
#                    guessed at here.
#
# Registered in scripts/gates.manifest: without that, this mechanism could vanish
# in a refactor exactly like the drifts it exists to catch.
#
# Manual run (bypasses the 15-minute throttle): `mise run cycle:check`.
set -euo pipefail

command -v jq >/dev/null 2>&1 || exit 0
command -v bd >/dev/null 2>&1 || exit 0
# The hook contract feeds JSON on stdin; drain it so the writer never blocks.
cat >/dev/null 2>&1 || true

repo="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$repo" 2>/dev/null || exit 0

# Throttle: these signals move on the scale of hours, not keystrokes.
# ENCRE_CYCLE_NOCACHE=1 bypasses it — used by the bite test and by
# `mise run cycle:check`.
if [[ -z "${ENCRE_CYCLE_NOCACHE:-}" ]]; then
  cache="${TMPDIR:-/tmp}/encre-cycle-check-$(id -u)"
  if [[ -f "$cache" ]]; then
    age=$(($(date +%s) - $(stat -c %Y "$cache" 2>/dev/null || echo 0)))
    [[ "$age" -lt 900 ]] && exit 0
  fi
  : >"$cache" 2>/dev/null || true
fi

due=""

# bd JSON payloads can exceed the OS argv limit, so they are handed to python3
# as file paths, never as argv strings.
work="$(mktemp -d "${TMPDIR:-/tmp}/encre-cycle-check.XXXXXX" 2>/dev/null || echo "")"
[[ -z "$work" ]] && exit 0
trap 'rm -rf "$work"' EXIT

bd list --status open --json -n 0 >"$work/open.json" 2>/dev/null || echo '[]' >"$work/open.json"

# ── Signal 1: éventail ────────────────────────────────────────────────────────
# More than 6 open P0/P1 epics means the priorities were each set on their own
# and never weighed against one another — seven epics each holding a P1 are not
# one plan, they are seven plans.
epics=$(python3 -c "
import sys, json
try:
    with open(sys.argv[1]) as f:
        rows = json.load(f)
    print(sum(1 for i in rows if i.get('issue_type') == 'epic' and i.get('priority', 9) <= 1))
except Exception:
    print(0)
" "$work/open.json" 2>/dev/null || echo 0)
if [[ "${epics:-0}" -gt 6 ]]; then
  due+="  • ÉVENTAIL : ${epics} epics ouverts en P0/P1 (seuil 6). L'ordre du brief doit se lire dans les priorités — rétrograder ce qui vient après, ou fusionner.\n"
fi

# ── Signal 2: boucle d'amélioration ───────────────────────────────────────────
# A bead closed on a PROMISE ("on surveillera", "tenu par convention") with no
# follow-up bead referencing it is a guarantee with no mechanism — the exact
# thing this project's doctrine refuses. The promise is looked for only in the
# CLOSING record (notes/close_reason), never in title/description: a bead whose
# description names the defect it just FIXED would otherwise nag forever, and a
# guardrail that cries wolf gets tuned out.
bd list --status closed --json -n 0 >"$work/closed.json" 2>/dev/null || echo '[]' >"$work/closed.json"
bd list --all --json -n 0 >"$work/all.json" 2>/dev/null || echo '[]' >"$work/all.json"
cands=$(python3 -c "
import sys, json, re
try:
    with open(sys.argv[1]) as f:
        closed = json.load(f)
    with open(sys.argv[2]) as f:
        allbeads = json.load(f)
    pat = re.compile(r'tenu par convention|held by convention|par vigilance|on pensera|on surveillera|for now|à surveiller', re.I)

    def text(i):
        return ' '.join(str(i.get(f, '') or '') for f in ('title', 'description', 'notes', 'close_reason'))

    def promise_text(i):
        return ' '.join(str(i.get(f, '') or '') for f in ('notes', 'close_reason'))

    def mentions(id_, body):
        return bool(re.search(r'(?<![\w.])' + re.escape(id_) + r'(?![\w.])', body))

    candidates = [i for i in closed if pat.search(promise_text(i))]
    referenced = set()
    for other in allbeads:
        body = text(other)
        for c in candidates:
            cid = c.get('id', '')
            if cid and cid != other.get('id') and mentions(cid, body):
                referenced.add(cid)
    print(len([c for c in candidates if c.get('id') not in referenced]))
except Exception:
    print(0)
" "$work/closed.json" "$work/all.json" 2>/dev/null || echo 0)
if [[ "${cands:-0}" -gt 0 ]]; then
  due+="  • BOUCLE D'AMÉLIORATION : ${cands} bead(s) clos sur une promesse (« tenu par convention », « on surveillera ») sans bead de suivi. Convertir la promesse en mécanisme, ou ouvrir le bead qui le fera.\n"
fi

# ── Signal 3: dérive du récit ─────────────────────────────────────────────────
# PROGRESS.md carries the narrative and the log; beads carry the tasks. If work
# keeps landing without the narrative ever being revisited, it has silently
# stopped describing the project.
if [[ -f PROGRESS.md ]]; then
  since=$(git log --oneline "$(git log -1 --format=%H -- PROGRESS.md 2>/dev/null)"..HEAD 2>/dev/null | wc -l | tr -d ' ')
  if [[ "${since:-0}" -gt 12 ]]; then
    due+="  • DÉRIVE DU RÉCIT : ${since} commits depuis la dernière révision de PROGRESS.md. Vérifier qu'il décrit encore l'état réel — c'est lui qu'on relit pour reprendre le projet.\n"
  fi
fi

# ── Signal 5: hiérarchie ──────────────────────────────────────────────────────
# Every bead lives under one of the head epics (`bd list -t epic`). An open
# non-epic bead with no parent has fallen out of that hierarchy; the graph was
# built with zero orphans on 2026-09-13 and this is what stops it eroding back.
hierarchie="$(python3 -c "
import sys, json
try:
    with open(sys.argv[1]) as f:
        rows = json.load(f)
    ids = [i.get('id', '') for i in rows
           if i.get('issue_type') != 'epic' and not (i.get('parent') or '').strip()]
    print(len(ids))
    print(','.join(ids))
except Exception:
    print(0)
    print('')
" "$work/open.json" 2>/dev/null || printf '0\n\n')"
hierarchie_count="$(sed -n '1p' <<<"$hierarchie")"
hierarchie_ids="$(sed -n '2p' <<<"$hierarchie")"
if [[ "${hierarchie_count:-0}" -gt 0 ]]; then
  ids_shown="$hierarchie_ids"
  id_count=$(($(tr -c ',' ' ' <<<"$hierarchie_ids" | wc -w) + 1))
  if [[ "$id_count" -gt 10 ]]; then
    ids_shown="$(cut -d',' -f1-10 <<<"$hierarchie_ids")…"
  fi
  due+="  • HIÉRARCHIE : ${hierarchie_count} bead(s) ouvert(s) sans parent (${ids_shown}). Tout bead vit sous un epic — \`bd update <id> --parent <epic>\`.\n"
fi

# ── Signal 6: non réclamé ─────────────────────────────────────────────────────
# A bead named by a `Bead:` trailer in a recent commit and still 'open' means
# someone committed against it without claiming or closing it. The trailer match
# is deliberately looser than the commit-msg gate's, so commits landed with
# --no-verify or before the gate existed are still caught.
#
# EXCEPTION: a commit that OPENS a bead is not a commit that failed to close it.
# The commit-msg gate requires the id to exist before the commit can cite it, so
# `bd create` always runs shortly BEFORE the citing commit — a bead created less
# than OPEN_WINDOW_MINUTES before it is "opened by" that commit and excluded.
# COST, accepted: a bead genuinely delivered inside that same window escapes the
# signal; by timing alone it is indistinguishable from "just opened".
git log -30 --format='%H%x1f%cI%x1f%B%x1e' >"$work/log.txt" 2>/dev/null || : >"$work/log.txt"
non_reclame="$(python3 -c "
import sys, json, re
from datetime import datetime, timedelta, timezone

OPEN_WINDOW_MINUTES = 30

def parse_dt(raw):
    if not raw:
        return None
    s = raw.strip()
    if s.endswith('Z'):
        s = s[:-1] + '+00:00'
    try:
        dt = datetime.fromisoformat(s)
    except Exception:
        return None
    if dt.tzinfo is not None:
        dt = dt.astimezone(timezone.utc)
    return dt

try:
    with open(sys.argv[1], encoding='utf-8') as f:
        log = f.read()
    with open(sys.argv[2]) as f:
        allbeads = json.load(f)

    trailer_re = re.compile(r'^\s*Bead:\s*(.+)\$', re.I | re.M)
    id_re = re.compile(r'^encre-[a-z0-9]{3,8}(\.[0-9]+)*\$')

    cited = {}
    for rec in log.split('\x1e'):
        parts = rec.split('\x1f', 2)
        if len(parts) != 3:
            continue
        _h, commit_date_raw, body = parts
        commit_dt = parse_dt(commit_date_raw)
        for m in trailer_re.finditer(body):
            for tok in re.split(r'[,\s]+', m.group(1).strip()):
                if id_re.match(tok):
                    cited.setdefault(tok, []).append(commit_dt)

    window = timedelta(minutes=OPEN_WINDOW_MINUTES)
    by_id = {b.get('id'): b for b in allbeads}
    ids = []
    for id_, commit_dts in cited.items():
        bead = by_id.get(id_, {})
        if bead.get('status') != 'open':
            continue
        created = parse_dt(bead.get('created_at'))
        if created is None:
            ids.append(id_)  # unparseable creation date: fail loud, not silent
            continue
        if any(cd is None or created <= cd - window for cd in commit_dts):
            ids.append(id_)
    ids.sort()
    print(len(ids))
    print(','.join(ids))
except Exception:
    print(0)
    print('')
" "$work/log.txt" "$work/all.json" 2>/dev/null || printf '0\n\n')"
non_reclame_count="$(sed -n '1p' <<<"$non_reclame")"
non_reclame_ids="$(sed -n '2p' <<<"$non_reclame")"
if [[ "${non_reclame_count:-0}" -gt 0 ]]; then
  ids_shown="$non_reclame_ids"
  id_count=$(($(tr -c ',' ' ' <<<"$non_reclame_ids" | wc -w) + 1))
  if [[ "$id_count" -gt 10 ]]; then
    ids_shown="$(cut -d',' -f1-10 <<<"$non_reclame_ids")…"
  fi
  due+="  • NON RÉCLAMÉ : ${non_reclame_count} bead(s) cité(s) par un trailer Bead: dans les 30 derniers commits mais encore open (${ids_shown}) — commité contre eux sans les réclamer ni les clore. \`bd update <id> --claim\` si ça continue, \`bd close <id> -r '<hash> …'\` si c'est livré.\n"
fi

# ── Signal 7: processus ───────────────────────────────────────────────────────
# A hook that measures its own kind is a warning sign in itself: if most of what
# lands is hooks, gates and docs rather than the game, the process is eating the
# product it exists to protect. Product paths are those of brief/ENCRE_04 §3,
# plus any root-level .go file. A commit touching none of them — including a
# merge commit with no paths of its own — is process.
git log -20 --format=%H --name-only >"$work/commits.txt" 2>/dev/null || : >"$work/commits.txt"
processus_count="$(python3 -c "
import sys, re
PRODUCT = ('cmd/', 'engine/', 'lexique/', 'content/', 'server/', 'client/', 'sim/', 'internal/')
try:
    with open(sys.argv[1], encoding='utf-8') as f:
        lines = f.read().splitlines()

    hash_re = re.compile(r'^[0-9a-f]{40}\$')
    commits = []
    cur = None
    for line in lines:
        if hash_re.match(line):
            cur = []
            commits.append(cur)
        elif line.strip() and cur is not None:
            cur.append(line)

    def is_product(paths):
        return any(p.startswith(PRODUCT) or re.fullmatch(r'[^/]+\.go', p) for p in paths)

    print(sum(1 for c in commits if not is_product(c)))
except Exception:
    print(0)
" "$work/commits.txt" 2>/dev/null || echo 0)"
if [[ "${processus_count:-0}" -gt 10 ]]; then
  due+="  • PROCESSUS : ${processus_count} des 20 derniers commits ne touchent aucun paquet produit — le processus mange le produit. Le prochain travail est du jeu, dans l'ordre du backlog ; un gate de plus n'est pas une livraison.\n"
fi

# ── Signal 10: arbre en conflit ───────────────────────────────────────────────
# A conflicted working tree with no merge or rebase in progress: an old `git
# stash apply` landing in conflict never moves HEAD and writes nothing to the
# reflog, so the state is invisible to any inspection of history — which is
# exactly why a STATE signal is needed here. A real merge/rebase in progress is
# deliberate work, so stay silent when MERGE_HEAD or a rebase directory exists.
# `=======` alone is too common in ordinary prose (a Markdown rule) to use as a
# marker; `<<<<<<< ` and `>>>>>>> ` with their trailing space, as git writes
# them, are not.
git_dir="$(git rev-parse --git-dir 2>/dev/null || echo '')"
if [[ -n "$git_dir" && ! -f "$git_dir/MERGE_HEAD" && ! -d "$git_dir/rebase-merge" && ! -d "$git_dir/rebase-apply" ]]; then
  {
    git ls-files -u 2>/dev/null | cut -f2- || true
    git grep -I -l -e '^<<<<<<< ' -e '^>>>>>>> ' -- . 2>/dev/null || true
  } | sort -u >"$work/conflict.txt" 2>/dev/null || : >"$work/conflict.txt"
  conflict_count="$(wc -l <"$work/conflict.txt" 2>/dev/null | tr -d ' ')"
  if [[ "${conflict_count:-0}" -gt 0 ]]; then
    conflict_shown="$(head -n 10 "$work/conflict.txt" | tr '\n' ',' | sed 's/,$//')"
    [[ "$conflict_count" -gt 10 ]] && conflict_shown+="…"
    due+="  • ARBRE EN CONFLIT : ${conflict_count} fichier(s) suivi(s) en conflit — index non fusionné et/ou marqueurs <<<<<<< / >>>>>>> (${conflict_shown}). Aucun merge ni rebase en cours : cet état ne laisse aucune trace dans le reflog. Récupération : mettre de côté les versions en conflit, puis \`git checkout HEAD -- <chemin>\`.\n"
  fi
fi

[[ -z "$due" ]] && exit 0

jq -cn --arg c "$(printf 'CYCLE DE DÉVELOPPEMENT — un seuil est franchi (.claude/hooks/cycle-check.sh, état mesuré, pas mémorisé) :\n%b\nTraiter avant de lancer du travail neuf, ou dire explicitement pourquoi on diffère.' "$due")" \
  '{hookSpecificOutput:{hookEventName:"UserPromptSubmit",additionalContext:$c}}'
