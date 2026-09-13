#!/usr/bin/env bash
# Point git at the versioned hooks directory so .githooks/pre-commit runs on commit.
# Run once after cloning: `./scripts/install-hooks.sh`.
#
# If bd (beads) has installed its shims under .beads/hooks, point core.hooksPath
# there instead: those shims run .githooks/<name> FIRST (their exit code aborts
# the commit) and then bd's own integration block, so both chains stay live.
# Resetting hooksPath to .githooks here would silently drop bd's half — and
# pointing it at .beads/hooks without checking would silently drop the kit's
# gates if a future `bd init` ever writes non-delegating hooks. Hence the probe.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

if [[ -f .beads/hooks/pre-commit ]] && grep -qF '.githooks/pre-commit' .beads/hooks/pre-commit; then
  git config core.hooksPath .beads/hooks
  echo "✓ core.hooksPath set to .beads/hooks — its shims delegate to .githooks (kit gates + beads are both active)."
else
  git config core.hooksPath .githooks
  echo "✓ core.hooksPath set to .githooks — pre-commit style gate is active."
fi

chmod +x .githooks/* scripts/*.sh .claude/hooks/*.sh .beads/hooks/* 2>/dev/null || true
