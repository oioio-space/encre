package net_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/oioio-space/encre/client/net"
)

// fakeSender is a [net.Sender] driven entirely by outcomes, keyed by run ID,
// so queue tests never reach a real server. calls records every job it was
// asked to send, in order, for tests asserting on that order.
type fakeSender struct {
	outcomes map[string]net.Result
	errs     map[string]error
	calls    []string
}

func (f *fakeSender) SendFinish(_ context.Context, job net.Job) (net.Result, error) {
	f.calls = append(f.calls, job.RunID)
	if err, ok := f.errs[job.RunID]; ok {
		return 0, err
	}
	return f.outcomes[job.RunID], nil
}

func TestQueueEnqueueThenJobsReturnsWhatWasQueued(t *testing.T) {
	q := net.NewQueue(newMemStore())

	if err := q.Enqueue(net.Job{RunID: "run-1"}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := q.Enqueue(net.Job{RunID: "run-2"}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	jobs, err := q.Jobs()
	if err != nil {
		t.Fatalf("Jobs: %v", err)
	}
	if got, want := len(jobs), 2; got != want {
		t.Fatalf("len(Jobs()) = %d, want %d", got, want)
	}
	if jobs[0].RunID != "run-1" || jobs[1].RunID != "run-2" {
		t.Errorf("Jobs() = %v, want [run-1 run-2] in that order", jobs)
	}
}

func TestDrainSendsEveryJobInOrderAndEmptiesTheQueue(t *testing.T) {
	q := net.NewQueue(newMemStore())
	for _, id := range []string{"run-1", "run-2", "run-3"} {
		if err := q.Enqueue(net.Job{RunID: id}); err != nil {
			t.Fatalf("Enqueue(%s): %v", id, err)
		}
	}
	sender := &fakeSender{outcomes: map[string]net.Result{
		"run-1": net.ResultApplied,
		"run-2": net.ResultApplied,
		"run-3": net.ResultApplied,
	}}

	report, err := q.Drain(t.Context(), sender)
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}

	want := []string{"run-1", "run-2", "run-3"}
	if got := sender.calls; !slices.Equal(got, want) {
		t.Errorf("send order = %v, want %v", got, want)
	}
	if got := report.Delivered; !slices.Equal(got, want) {
		t.Errorf("report.Delivered = %v, want %v", got, want)
	}
	if report.Pending != 0 {
		t.Errorf("report.Pending = %d, want 0", report.Pending)
	}
	remaining, err := q.Jobs()
	if err != nil {
		t.Fatalf("Jobs: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("Jobs() after a full drain = %v, want empty", remaining)
	}
}

func TestDrainTreatsAConflictAsDeliveredNotAsAFailure(t *testing.T) {
	q := net.NewQueue(newMemStore())
	if err := q.Enqueue(net.Job{RunID: "run-1"}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	sender := &fakeSender{outcomes: map[string]net.Result{"run-1": net.ResultAlreadyApplied}}

	report, err := q.Drain(t.Context(), sender)
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}

	if len(report.Rejected) != 0 {
		t.Errorf("report.Rejected = %v, want empty: a 409 is a success, not a failure", report.Rejected)
	}
	if got, want := report.Delivered, []string{"run-1"}; !slices.Equal(got, want) {
		t.Errorf("report.Delivered = %v, want %v", got, want)
	}
}

func TestDrainStopsAtAnUnreachableJobAndKeepsItAndLaterJobsPending(t *testing.T) {
	q := net.NewQueue(newMemStore())
	for _, id := range []string{"run-1", "run-2", "run-3"} {
		if err := q.Enqueue(net.Job{RunID: id}); err != nil {
			t.Fatalf("Enqueue(%s): %v", id, err)
		}
	}
	sender := &fakeSender{outcomes: map[string]net.Result{
		"run-1": net.ResultApplied,
		"run-2": net.ResultUnreachable,
		"run-3": net.ResultApplied,
	}}

	report, err := q.Drain(t.Context(), sender)
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}

	if got, want := sender.calls, []string{"run-1", "run-2"}; !slices.Equal(got, want) {
		t.Errorf("send order = %v, want %v: run-3 must wait behind the unreachable run-2", got, want)
	}
	if got, want := report.Delivered, []string{"run-1"}; !slices.Equal(got, want) {
		t.Errorf("report.Delivered = %v, want %v", got, want)
	}
	if report.Pending != 2 {
		t.Errorf("report.Pending = %d, want 2 (run-2 and run-3)", report.Pending)
	}
	remaining, err := q.Jobs()
	if err != nil {
		t.Fatalf("Jobs: %v", err)
	}
	if got, want := jobIDs(remaining), []string{"run-2", "run-3"}; !slices.Equal(got, want) {
		t.Errorf("Jobs() after drain = %v, want %v, in order", got, want)
	}
}

func TestDrainDropsAPermanentlyRejectedJobAndContinues(t *testing.T) {
	q := net.NewQueue(newMemStore())
	for _, id := range []string{"run-1", "run-2"} {
		if err := q.Enqueue(net.Job{RunID: id}); err != nil {
			t.Fatalf("Enqueue(%s): %v", id, err)
		}
	}
	sender := &fakeSender{outcomes: map[string]net.Result{
		"run-1": net.ResultRejected,
		"run-2": net.ResultApplied,
	}}

	report, err := q.Drain(t.Context(), sender)
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}

	if got, want := report.Rejected, []string{"run-1"}; !slices.Equal(got, want) {
		t.Errorf("report.Rejected = %v, want %v", got, want)
	}
	if got, want := report.Delivered, []string{"run-2"}; !slices.Equal(got, want) {
		t.Errorf("report.Delivered = %v, want %v: a rejection must not block later jobs", got, want)
	}
	if report.Pending != 0 {
		t.Errorf("report.Pending = %d, want 0", report.Pending)
	}
}

func TestDrainOfAnEmptyQueueDoesNothing(t *testing.T) {
	q := net.NewQueue(newMemStore())
	sender := &fakeSender{}

	report, err := q.Drain(t.Context(), sender)
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if len(sender.calls) != 0 {
		t.Errorf("sender.calls = %v, want none", sender.calls)
	}
	if report.Pending != 0 {
		t.Errorf("report.Pending = %d, want 0", report.Pending)
	}
}

var errBoom = errors.New("boom")

func TestDrainPropagatesASenderError(t *testing.T) {
	q := net.NewQueue(newMemStore())
	if err := q.Enqueue(net.Job{RunID: "run-1"}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	sender := &fakeSender{errs: map[string]error{"run-1": errBoom}}

	if _, err := q.Drain(t.Context(), sender); !errors.Is(err, errBoom) {
		t.Errorf("Drain error = %v, want it to wrap %v", err, errBoom)
	}
}

func jobIDs(jobs []net.Job) []string {
	ids := make([]string, len(jobs))
	for i, j := range jobs {
		ids[i] = j.RunID
	}
	return ids
}
