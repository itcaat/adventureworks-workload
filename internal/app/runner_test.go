package app

import (
	"context"
	"database/sql"
	"math/rand"
	"testing"
	"time"
)

func TestStartDelaySpreadsUsersAcrossRamp(t *testing.T) {
	cfg := Config{Users: 4, Ramp: 12 * time.Second}

	tests := []struct {
		userID int
		want   time.Duration
	}{
		{userID: 1, want: 0},
		{userID: 2, want: 3 * time.Second},
		{userID: 3, want: 6 * time.Second},
		{userID: 4, want: 9 * time.Second},
	}

	for _, tt := range tests {
		got := startDelay(cfg, tt.userID)
		if got != tt.want {
			t.Fatalf("startDelay(user=%d) = %s, want %s", tt.userID, got, tt.want)
		}
	}
}

func TestStartDelayDisabledForSingleUserOrZeroRamp(t *testing.T) {
	if got := startDelay(Config{Users: 1, Ramp: 10 * time.Second}, 1); got != 0 {
		t.Fatalf("single user delay = %s, want 0", got)
	}
	if got := startDelay(Config{Users: 10, Ramp: 0}, 10); got != 0 {
		t.Fatalf("zero ramp delay = %s, want 0", got)
	}
}

func TestRunUserDrainsInFlightOperation(t *testing.T) {
	scheduleCtx, stopSchedule := context.WithCancel(context.Background())
	opBase := context.Background()

	started := make(chan struct{})
	ops := []Operation{{
		Name:   "slow",
		Kind:   "read",
		Weight: 1,
		Run: func(ctx context.Context, db *sql.DB, rng *rand.Rand, p Persona) (TrafficStats, error) {
			close(started)
			select {
			case <-time.After(50 * time.Millisecond):
				return TrafficStats{Sent: 128, Received: 256}, nil
			case <-ctx.Done():
				return TrafficStats{}, ctx.Err()
			}
		},
	}}

	cfg := Config{ThinkMin: 0, ThinkMax: 0, RequestTimeout: time.Second}
	recorder := NewRecorder("test", cfg, ops)
	tracker := &userTracker{}

	done := make(chan struct{})
	go func() {
		runUser(scheduleCtx, opBase, nil, cfg, recorder, tracker, ops, 1)
		close(done)
	}()

	<-started
	stopSchedule()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runUser did not wait for in-flight operation")
	}

	if snap := recorder.Snapshot(); snap.TotalErrors != 0 {
		t.Fatalf("TotalErrors = %d, want 0", snap.TotalErrors)
	}
}

func TestRunUserDrainsInFlightOperationAfterDurationTimeout(t *testing.T) {
	parent := context.Background()
	scheduleCtx, _ := context.WithTimeout(parent, 20*time.Millisecond)
	opBase, opCancel := context.WithCancel(parent)
	defer opCancel()

	opStarted := make(chan struct{})
	ops := []Operation{{
		Name:   "slow",
		Kind:   "read",
		Weight: 1,
		Run: func(ctx context.Context, db *sql.DB, rng *rand.Rand, p Persona) (TrafficStats, error) {
			close(opStarted)
			select {
			case <-time.After(80 * time.Millisecond):
				if ctx.Err() != nil {
					return TrafficStats{}, ctx.Err()
				}
				return TrafficStats{Sent: 1, Received: 1}, nil
			case <-ctx.Done():
				return TrafficStats{}, ctx.Err()
			}
		},
	}}

	cfg := Config{ThinkMin: 0, ThinkMax: 0, RequestTimeout: time.Second}
	recorder := NewRecorder("test", cfg, ops)
	tracker := &userTracker{}

	done := make(chan struct{})
	go func() {
		runUser(scheduleCtx, opBase, nil, cfg, recorder, tracker, ops, 1)
		close(done)
	}()

	<-opStarted
	<-scheduleCtx.Done()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runUser did not wait for in-flight operation after duration timeout")
	}

	if snap := recorder.Snapshot(); snap.TotalErrors != 0 {
		t.Fatalf("TotalErrors = %d, want 0", snap.TotalErrors)
	}
}

func TestOpBaseNotCancelledWhenScheduleEnds(t *testing.T) {
	parent := context.Background()
	scheduleCtx, _ := context.WithTimeout(parent, 20*time.Millisecond)
	opBase, opCancel := context.WithCancel(context.Background())
	defer opCancel()
	go func() {
		<-parent.Done()
		opCancel()
	}()

	<-scheduleCtx.Done()
	time.Sleep(10 * time.Millisecond)

	if scheduleCtx.Err() != context.DeadlineExceeded {
		t.Fatalf("schedule err = %v, want deadline exceeded", scheduleCtx.Err())
	}
	if opBase.Err() != nil {
		t.Fatalf("opBase cancelled when schedule ended: %v", opBase.Err())
	}
}
