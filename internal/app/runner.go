package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"
)

func Run(parent context.Context, db *sql.DB, cfg Config, logger *slog.Logger, onProgress ProgressFunc) (Report, error) {
	runID := cfg.RunID()
	ops := buildOperations(cfg, runID)
	if len(ops) == 0 {
		return Report{}, fmt.Errorf("no operations enabled for profile=%s write-mode=%s", cfg.Profile, cfg.WriteMode)
	}

	workloadCtx, workloadCancel := context.WithTimeout(parent, cfg.Duration)
	defer workloadCancel()

	// Operation contexts are not tied to workload duration so in-flight SQL can finish
	// after the configured run window ends instead of failing with deadline exceeded.
	opBase, opCancel := context.WithCancel(parent)
	defer opCancel()

	recorder := NewRecorder(runID, cfg, ops)
	tracker := &userTracker{}
	started := time.Now()
	logger.Info("starting workload",
		"run_id", runID,
		"users", cfg.Users,
		"duration", cfg.Duration,
		"profile", cfg.Profile,
		"write_mode", cfg.WriteMode,
	)

	drainLogged := make(chan struct{})
	go func() {
		defer close(drainLogged)
		<-workloadCtx.Done()
		if workloadCtx.Err() == context.DeadlineExceeded {
			logger.Info("workload duration elapsed, draining in-flight operations")
		}
	}()

	var wg sync.WaitGroup
	for userID := 1; userID <= cfg.Users; userID++ {
		delay := startDelay(cfg, userID)
		wg.Add(1)
		go func(id int, delay time.Duration) {
			defer wg.Done()
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-workloadCtx.Done():
				return
			case <-timer.C:
			}
			runUser(workloadCtx, opBase, db, cfg, recorder, tracker, ops, id)
		}(userID, delay)
	}

	progressStop := make(chan struct{})
	progressDone := make(chan struct{})
	go reportProgress(parent, workloadCtx, recorder, tracker, cfg, started, logger, onProgress, progressStop, progressDone)

	wg.Wait()
	<-drainLogged
	close(progressStop)
	<-progressDone

	ended := time.Now()
	report := recorder.Report(started, ended)
	logger.Info("workload finished",
		"run_id", runID,
		"elapsed", ended.Sub(started).Round(time.Millisecond),
		"operations", report.TotalOperations,
		"errors", report.TotalErrors,
		"ops_per_sec", fmt.Sprintf("%.2f", report.OperationsPerSecond),
	)
	return report, parent.Err()
}

func runUser(workloadCtx, opBase context.Context, db *sql.DB, cfg Config, recorder *Recorder, tracker *userTracker, ops []Operation, userID int) {
	tracker.add(1)
	defer tracker.add(-1)

	rng := rand.New(rand.NewSource(cfg.Seed + int64(userID)*7919))
	persona := newPersona(userID, rng)

	for {
		select {
		case <-workloadCtx.Done():
			return
		default:
		}

		op := chooseOperation(ops, persona, rng)
		opCtx, cancel := context.WithTimeout(opBase, cfg.RequestTimeout)
		started := time.Now()
		traffic, err := op.Run(opCtx, db, rng, persona)
		cancel()
		recorder.Record(op.Name, op.Kind, persona.Type, time.Since(started), err, traffic)

		select {
		case <-workloadCtx.Done():
			return
		default:
		}

		delay := thinkDuration(cfg, persona, rng)
		timer := time.NewTimer(delay)
		select {
		case <-workloadCtx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func startDelay(cfg Config, userID int) time.Duration {
	if cfg.Ramp <= 0 || cfg.Users == 1 {
		return 0
	}
	step := cfg.Ramp / time.Duration(cfg.Users)
	return time.Duration(userID-1) * step
}

func reportProgress(parent, workloadCtx context.Context, recorder *Recorder, tracker *userTracker, cfg Config, started time.Time, logger *slog.Logger, onProgress ProgressFunc, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	if cfg.ProgressEvery <= 0 && onProgress == nil {
		select {
		case <-parent.Done():
		case <-stop:
		}
		return
	}
	interval := cfg.ProgressEvery
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	emit := func(phase RunPhase) {
		elapsed := time.Since(started)
		snap := recorder.LiveSnapshot(elapsed, phase, tracker.count())
		if onProgress != nil {
			onProgress(snap)
			return
		}
		if logger != nil {
			logger.Info("progress",
				"operations", snap.TotalOperations,
				"errors", snap.TotalErrors,
				"ops_per_sec", fmt.Sprintf("%.2f", snap.OperationsPerSecond),
				"p95_ms", fmt.Sprintf("%.1f", float64(snap.P95)/float64(time.Millisecond)),
				"active_users", snap.ActiveUsers,
				"phase", snap.Phase,
			)
		}
	}

	for {
		select {
		case <-parent.Done():
			return
		case <-stop:
			emit(PhaseDone)
			return
		case <-ticker.C:
			elapsed := time.Since(started)
			emit(computePhase(elapsed, cfg, workloadCtx))
		}
	}
}
