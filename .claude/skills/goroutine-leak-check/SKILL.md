---
name: goroutine-leak-check
description: Use when adding/changing concurrent Go (goroutines, channels, worker fan-out, context cancellation) or when asked to find/verify/fix goroutine leaks — instruments tests with uber-go/goleak (and the Go 1.26 goroutineleak profile), runs the suite, diagnoses leaks against known patterns, and fixes the production code.
---

# Goroutine-leak verification & repair

Any code that fans out across goroutines (worker pools, pipelines, fan-out/fan-in) and streams
results over a channel can leak: a goroutine blocked forever on a channel send/receive or a lock
no runnable goroutine can release. A leaked goroutine is a real correctness and memory bug. This
skill verifies there are none, and fixes any it finds.

State of the art (2026): **[uber-go/goleak](https://github.com/uber-go/goleak)** at test
boundaries (the portable default), and **Go 1.26's experimental `goroutineleak` pprof
profile** (GC-based, *no false positives* — it reports only goroutines it can prove are
stuck). This project is on Go 1.26, so both are available.

## Checklist (create a todo per item)

1. **Instrument the concurrent packages** with goleak at the test boundary, as soon as a package
   introduces concurrency (`go func`, `sync.WaitGroup`, `errgroup`, worker fan-out).
2. **Run the suite** (`mise run test`) and collect every leak goleak reports.
3. **Diagnose** each leak against the known patterns below (real leak vs. test artifact).
4. **Fix the production code** (not the test) for real leaks; suppress only proven
   framework goroutines, narrowly and with a reason.
5. **Re-run** until clean.
6. Keep the gate: because a per-package `TestMain` activates goleak on *every* `go test` run,
   `mise run test` and `mise run ci` (via `test:ci`) already detect leaks for free — no separate
   task is needed. For a large or slow suite, `scripts/gotest-caged.sh` runs it inside a
   memory-capped cgroup (see below) so a leak or runaway allocation can't take the machine down.

## 1. Instrument with goleak

Add the dependency (pure Go, no CGO — allowed):

```bash
go get go.uber.org/goleak
```

For each package that starts goroutines, add **one** `TestMain` so the check runs after
the whole package's tests (this is compatible with `t.Parallel`, unlike a per-test
`defer goleak.VerifyNone(t)`):

```go
package mypkg

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
```

Find the packages that need it with:
`grep -rln 'go func\|sync.WaitGroup\|errgroup' --include='*.go' . | grep -v _test.go`

If a package already has a `TestMain`, fold the goleak verification in around the
existing `m.Run()` and propagate its exit code; do not add a second one.

Per-test, sequential checks can use `defer goleak.VerifyNone(t)` — but **never** with
`t.Parallel`.

### Go 1.26 profile (deeper, no false positives)

For a hard-to-pin leak, capture the GC-backed profile (proves stuck goroutines):

```go
import "runtime/pprof"
pprof.Lookup("goroutineleak").WriteTo(os.Stderr, 1) // Go 1.26+, experimental
```

Use it to confirm a goleak finding is genuinely blocked (and on what) before fixing.

## 2. Run

Run the suite via `mise run test` (or `mise run ci`, which runs `test:ci`) — the per-package
`TestMain` makes goleak run for free on every invocation. For a heavy or long-running suite,
route it through the memory-capped cage instead of a bare `go test`, so a genuine leak or
runaway allocation is killed at the cage wall instead of exhausting the machine:

```bash
mise run test                                        # normal suite, goleak included
scripts/gotest-caged.sh go test ./internal/worker     # a single heavy package, caged
```

goleak needs no special flags — it runs via `TestMain`. A leak prints as
`found unexpected goroutines` with each one's stack — the bottom frame is the `go func` that
leaked; the top frame is where it is blocked.

> **No `-race` in the gate.** The race detector requires `CGO_ENABLED=1`; if this project is
> pure Go / no-CGO (see `CLAUDE.md`), goleak still detects *leaks* without `-race`. Use `-race`
> only as a manual, local-only deep check for *data races* (a different class of bug):
> `CGO_ENABLED=1 scripts/gotest-caged.sh go test -race ./...` — never wire it into a committed
> task or the CI gates on a no-CGO project.

## 3. Common leak patterns

- **Unconsumed results/progress channel.** A function that returns `(<-chan Event, <-chan Result)`
  must not do a *blocking* send on a channel the caller stopped reading. High-frequency events
  should be drop-on-full (`select … default`); terminal sends must `select` on `ctx.Done()`, or
  the channel must be buffered enough that the send can't block after the consumer is gone.
- **Worker fan-out without join.** Each fan-out must `wg.Wait()` (or errgroup `Wait()`) on
  **every** return path, including early error/`ctx` cancellation. A worker blocked on a full
  results channel after the collector returned is a leak — give the results channel enough
  buffer, or have workers `select` on `ctx.Done()`.
- **Context not honored.** A goroutine looping on work must check `ctx.Done()`; a
  `time.After`/`ticker` must be stopped. Store no `context.Context` in a struct.
- **Test-only background goroutines.** A goroutine the *test* starts (e.g. draining a channel)
  must finish before the test returns, or goleak attributes it to the package.

## 4. Fixing

Fix the **production** goroutine's lifecycle, not the test. The correct shapes:

- Owner holds the goroutine and exposes `Close`/`Stop`, or the goroutine is bounded by a
  `WaitGroup` the spawning function waits on before returning.
- Every channel a goroutine sends on is either buffered enough that the send can't block
  after consumers stop, or the send is `select { case ch<-v: case <-ctx.Done(): }`.
- Cancellation (`ctx`) unblocks every goroutine; the function that created the `ctx`
  (or derived a `cancel`) calls `cancel` on all paths (`defer cancel()`).

Suppress only a *proven* framework goroutine, narrowly:

```go
goleak.VerifyTestMain(m, goleak.IgnoreTopFunction("<pkg>.<func>")) // reason: …
```

## Done when

`mise run test` (or `mise run ci`) is clean, and every real leak was fixed in production code
(suppressions, if any, name a proven-external goroutine and a reason).
