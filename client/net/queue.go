package net

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/oioio-space/encre/engine"
)

// outboxKey is the Store key the queue of unsent finish requests is saved
// under.
const outboxKey = "outbox"

// Job is one POST /run/{id}/finish request waiting to be sent (ENCRE_04 §2,
// §7): everything the endpoint's body needs, plus the run's ID for the URL.
type Job struct {
	RunID     string
	Attempts  []engine.Attempt
	Talismans []engine.TalismanID
	Revanche  [3]bool
	Cahier    bool
}

// Result is how the server took one [Job], as [Sender.SendFinish] classifies
// it. It is the axis the whole queue turns on: ResultApplied and
// ResultAlreadyApplied both drop the job and move on, ResultRejected drops it
// without retrying, and only ResultUnreachable is worth trying again.
type Result int

const (
	// ResultApplied is a 200: the server ran the run's effects just now.
	ResultApplied Result = iota
	// ResultAlreadyApplied is a 409: another send of the same job already
	// got there first. POST /run/{id}/finish is idempotent (server/api/run.go,
	// store.Store.MarkApplied), so this is a success to the queue, not a
	// failure to report or a reason to resend.
	ResultAlreadyApplied
	// ResultRejected is a 4xx other than 409: the request itself is wrong
	// (an invalid run, a malformed body) and will never succeed by retrying
	// it unchanged.
	ResultRejected
	// ResultUnreachable is a timeout, a dropped connection, or a 5xx: the
	// server may not have seen the request at all, so it is worth trying
	// again later.
	ResultUnreachable
)

// Sender sends one [Job]'s finish request and reports what happened.
// [HTTPSender] is the production implementation; tests use a fake.
type Sender interface {
	SendFinish(ctx context.Context, job Job) (Result, error)
}

// Queue is the durable outbox of finish requests that could not be sent yet,
// saved through a [Store] so it survives a closed tab or a killed process
// the same way the run state does.
type Queue struct {
	store Store
}

// NewQueue returns a Queue persisting through store.
func NewQueue(store Store) *Queue {
	return &Queue{store: store}
}

// Jobs returns the jobs currently queued, oldest first.
//
// An absent or corrupted outbox both come back as a nil slice rather than an
// error, on the same reasoning as [LoadRunState]: a save that was never
// written and one that was written badly look identical from here, and a
// child must never be stuck behind a dead outbox that a real storage error
// (a full disk, in a native build) would otherwise turn into forever.
func (q *Queue) Jobs() ([]Job, error) {
	data, err := q.store.Load(outboxKey)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("net: reading the outbox: %w", err)
	}
	var jobs []Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, nil
	}
	return jobs, nil
}

// Enqueue appends job to the outbox, to be sent by the next [Queue.Drain].
func (q *Queue) Enqueue(job Job) error {
	jobs, err := q.Jobs()
	if err != nil {
		return err
	}
	return q.save(append(jobs, job))
}

func (q *Queue) save(jobs []Job) error {
	data, err := json.Marshal(jobs)
	if err != nil {
		return fmt.Errorf("net: encoding the outbox: %w", err)
	}
	if err := q.store.Save(outboxKey, data); err != nil {
		return fmt.Errorf("net: saving the outbox: %w", err)
	}
	return nil
}

// DrainReport summarizes what one [Queue.Drain] did.
type DrainReport struct {
	// Delivered lists the run IDs the server accepted this drain, whether
	// applied just now or already applied by an earlier send.
	Delivered []string
	// Rejected lists the run IDs the server permanently refused: dropped
	// from the queue, not retried.
	Rejected []string
	// Pending is how many jobs remain queued after this drain, waiting on
	// the network.
	Pending int
}

// Drain sends every queued job through sender, in order, and persists
// whatever is left afterwards.
//
// Jobs are sent oldest first and Drain stops at the first
// [ResultUnreachable]: that job and everything queued after it stay pending,
// so a later run's finish never overtakes an earlier one still waiting on
// the network. ResultApplied and ResultAlreadyApplied both remove a job and
// let Drain continue; a 409 is exactly as much a success as a 200 here (see
// [ResultAlreadyApplied]). ResultRejected also removes the job — retrying an
// invalid request would only fail again — and is reported separately so a
// caller can tell the two kinds of "gone from the queue" apart.
func (q *Queue) Drain(ctx context.Context, sender Sender) (DrainReport, error) {
	jobs, err := q.Jobs()
	if err != nil {
		return DrainReport{}, err
	}

	var report DrainReport
	stopped := len(jobs)
loop:
	for i, job := range jobs {
		result, err := sender.SendFinish(ctx, job)
		if err != nil {
			return report, fmt.Errorf("net: sending run %s: %w", job.RunID, err)
		}
		switch result {
		case ResultApplied, ResultAlreadyApplied:
			report.Delivered = append(report.Delivered, job.RunID)
		case ResultRejected:
			report.Rejected = append(report.Rejected, job.RunID)
		case ResultUnreachable:
			stopped = i
			break loop
		default:
			return report, fmt.Errorf("net: sender returned an unknown result %d for run %s", result, job.RunID)
		}
	}

	remaining := jobs[stopped:]
	report.Pending = len(remaining)
	if err := q.save(remaining); err != nil {
		return report, fmt.Errorf("net: persisting the outbox after drain: %w", err)
	}
	return report, nil
}
