# encre — project guidance

State & roadmap: see `PROGRESS.md`. Tooling is managed by **mise** — run everything with
`mise run …` (lint, test, ci, fmt, scan:*, sbom, …). Commits go through git hooks
(janitor → secrets → vulns → style → **/simplify review attestation**) and Claude
PreToolUse hooks (style/simplify/secret/vuln review, modern-Go injection on `.go` edits).
The `/simplify` review is a **mandatory gate on every commit path**: `.githooks/pre-commit`
requires the approval marker (`$GIT_DIR/claude-simplify-ok`, keyed to the exact staged
diff) — arm it only after genuinely completing the review; `--no-verify` bypasses
(emergency only).

## Commands (all via mise)

```bash
mise run setup           # bootstrap toolchain + wire git hooks
mise run test            # unit tests        | mise run test:watch  (TDD)
mise run lint            # golangci-lint v2  | mise run fmt
mise run cover:check     # coverage gate (COVER_MIN=85)
mise run ci              # full gate = what CI runs (lint+test+scans)
mise run bench:baseline  # then change code, then: mise run bench:compare (benchstat)
mise run scan:code       # gosec + govulncheck | scan:secrets | scan:sbom (grype)
mise run clean           # remove regenerable artifacts
```

## Architecture

Module `github.com/oioio-space/encre` — <décrire ici l'architecture : packages, responsabilités,
frontières. Remplir dès les premières briques ; ce paragraphe guide tous les agents.>

- Package racine : API de la bibliothèque. `cmd/encre` — CLI.
- `internal/` — packages internes (créer par responsabilité, un package = un rôle).

## ⛔ Absolute rule: NO CGO

This project is **pure Go — CGO is forbidden, no exceptions.** Never add `import "C"`, a
cgo-requiring dependency, or anything that needs a C toolchain. `CGO_ENABLED=0` is pinned
in `mise.toml [env]` and enforced by the `cgo:check` gate (part of `ci` and the pre-commit
gate). Pick pure-Go libraries only. If a task seems to need CGO, find a pure-Go path or
stop and raise it — do not relax the rule.

## ⚡ Benchmark the hot path & prove perf changes

Any perf-affecting change to a hot-path package is proven with benchstat — never by feel
(go-benchmark skill): `mise run bench:baseline` → change → `mise run bench:compare`
(`-count` ≥ 10, `-benchmem`); keep the change only on a statistically significant gain
with no alloc/throughput regression. Hot-path packages carry `Benchmark…` tests.
List them on the line below (space-separated package paths; read by
`.claude/hooks/benchmark-context.sh` — keep the exact `HOT_PATHS:` prefix):

HOT_PATHS:

## Research-first

Ground non-trivial / technical answers in CURRENT external sources before answering —
WebSearch/WebFetch for the state of the art, GitHub (`scripts/ghx.sh`) for prior art &
libraries, official docs. **Then go beyond the existing**: critique it, look for
improvements, and consider out-of-the-box / novel approaches. Prefer sources over memory,
compare alternatives, recommend the best (even if uncommon), cite. A `UserPromptSubmit`
hook (`.claude/hooks/research-grounding.sh`) reinforces this every prompt; skip for
trivial asks.

## Sub-agent routing (token-economical, no quality loss)

Each agent in `.claude/agents/` pins its own tier via frontmatter.

| Task | Agent | Model / effort |
|------|-------|----------------|
| Run checks (lint/test/ci/scan), trivial format fixes | `quality-runner` | Haiku / low |
| Docs, PROGRESS.md, commit messages | `scribe` | Haiku / low |
| Code/file search, fan-out, "where is X" | `explorer` (or built-in `Explore`) | Haiku / low |
| Implement Go code & tests, refactors | `go-dev` | Sonnet / medium |
| Review a Go diff | `go-reviewer` | Sonnet / medium |
| Novel algorithm / architecture design | `architect` | Opus / high |
| Deep security / vuln audit | `security-auditor` | Opus / high |

Rule of thumb: mechanical → Haiku; writing/reviewing Go → Sonnet; novel algorithm design
or security judgement → Opus. Never use Opus for what a cheaper tier handles at equal
quality. Run independent sub-tasks in parallel.

### Review-hook → agent routing

The AI-review PreToolUse hooks (fire on `git commit`) name the cost-appropriate agent for
a **substantial** diff; a trivial diff is cleared inline:

| Hook | Delegate to (tier) |
|------|--------------------|
| `commit-style-review` | `go-reviewer` (Sonnet) |
| `commit-cleanup-review` | `quality-runner` (Haiku) |
| `commit-secret-review` | `security-auditor` (Opus, only if ambiguous) |
| `commit-vuln-review` | `security-auditor` (Opus) |
| `commit-ergonomics-review` | `go-reviewer` (Sonnet) review → `go-dev` implement |
| `commit-docs-review` | `scribe` (Haiku) README/PROGRESS sync |


<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:6cd5cc61 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->
