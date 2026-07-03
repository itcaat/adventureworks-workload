package app

import (
	"context"
	"sync/atomic"
	"time"
)

type RunPhase string

const (
	PhaseRamping  RunPhase = "ramping"
	PhaseRunning  RunPhase = "running"
	PhaseDraining RunPhase = "draining"
	PhaseStopping RunPhase = "stopping"
	PhaseDone     RunPhase = "done"
)

type ProgressFunc func(LiveSnapshot)

type OperationLive struct {
	Name          string
	Kind          string
	Count         int64
	Errors        int64
	ErrorRate     float64
	P95           time.Duration
	BytesSent     int64
	BytesReceived int64
}

type LiveSnapshot struct {
	RunID     string
	Profile   string
	WriteMode string
	Users     int
	Duration  time.Duration
	Ramp      time.Duration
	Elapsed   time.Duration
	Phase     RunPhase

	ActiveUsers int

	TotalOperations     int64
	TotalErrors         int64
	OperationsPerSecond float64
	P50                 time.Duration
	P95                 time.Duration
	P99                 time.Duration

	BytesSent              int64
	BytesReceived          int64
	BytesSentPerSecond     float64
	BytesReceivedPerSecond float64

	Operations    []OperationLive
	Personas      map[string]int64
	Timeline []TimelineBucket
}

func computePhase(elapsed time.Duration, cfg Config, workloadCtx context.Context) RunPhase {
	select {
	case <-workloadCtx.Done():
		if workloadCtx.Err() == context.DeadlineExceeded {
			return PhaseDraining
		}
		return PhaseStopping
	default:
	}

	if cfg.Ramp > 0 && elapsed < cfg.Ramp {
		return PhaseRamping
	}
	return PhaseRunning
}

type userTracker struct {
	active atomic.Int64
}

func (t *userTracker) add(delta int64) {
	t.active.Add(delta)
}

func (t *userTracker) count() int {
	return int(t.active.Load())
}

func estimatedRampUsers(cfg Config, elapsed time.Duration) int {
	if cfg.Ramp <= 0 || cfg.Users == 1 {
		return cfg.Users
	}
	if elapsed >= cfg.Ramp {
		return cfg.Users
	}
	step := cfg.Ramp / time.Duration(cfg.Users)
	if step <= 0 {
		return cfg.Users
	}
	started := int(elapsed/step) + 1
	return min(started, cfg.Users)
}
