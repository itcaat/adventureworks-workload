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
	workloadCtx, stopWorkload := context.WithCancel(context.Background())
	opBase := context.Background()

	started := make(chan struct{})
	ops := []Operation{{
		Name:   "slow",
		Kind:   "read",
		Weight: 1,
		Run: func(ctx context.Context, db *sql.DB, rng *rand.Rand, p Persona) error {
			close(started)
			select {
			case <-time.After(50 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}}

	cfg := Config{ThinkMin: 0, ThinkMax: 0, RequestTimeout: time.Second}
	recorder := NewRecorder("test", cfg, ops)

	done := make(chan struct{})
	go func() {
		runUser(workloadCtx, opBase, nil, cfg, recorder, ops, 1)
		close(done)
	}()

	<-started
	stopWorkload()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runUser did not wait for in-flight operation")
	}

	if snap := recorder.Snapshot(); snap.TotalErrors != 0 {
		t.Fatalf("TotalErrors = %d, want 0", snap.TotalErrors)
	}
}
