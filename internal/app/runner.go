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

func Run(parent context.Context, db *sql.DB, cfg Config, logger *slog.Logger) (Report, error) {
	runID := cfg.RunID()
	ops := buildOperations(cfg, runID)
	if len(ops) == 0 {
		return Report{}, fmt.Errorf("no operations enabled for profile=%s write-mode=%s", cfg.Profile, cfg.WriteMode)
	}

	ctx, cancel := context.WithTimeout(parent, cfg.Duration)
	defer cancel()

	recorder := NewRecorder(runID, cfg, ops)
	started := time.Now()
	logger.Info("starting workload",
		"run_id", runID,
		"users", cfg.Users,
		"duration", cfg.Duration,
		"profile", cfg.Profile,
		"write_mode", cfg.WriteMode,
	)

	var wg sync.WaitGroup
	for userID := 1; userID <= cfg.Users; userID++ {
		delay := startDelay(cfg, userID)
		wg.Add(1)
		go func(id int, delay time.Duration) {
			defer wg.Done()
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			runUser(ctx, db, cfg, recorder, ops, id)
		}(userID, delay)
	}

	progressDone := make(chan struct{})
	go logProgress(ctx, recorder, cfg, logger, progressDone)

	wg.Wait()
	cancel()
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

func runUser(ctx context.Context, db *sql.DB, cfg Config, recorder *Recorder, ops []Operation, userID int) {
	rng := rand.New(rand.NewSource(cfg.Seed + int64(userID)*7919))
	persona := newPersona(userID, rng)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		op := chooseOperation(ops, persona, rng)
		opCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
		started := time.Now()
		err := op.Run(opCtx, db, rng, persona)
		cancel()
		recorder.Record(op.Name, op.Kind, persona.Type, time.Since(started), err)

		delay := thinkDuration(cfg, persona, rng)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
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

func logProgress(ctx context.Context, recorder *Recorder, cfg Config, logger *slog.Logger, done chan<- struct{}) {
	defer close(done)
	if cfg.ProgressEvery <= 0 {
		<-ctx.Done()
		return
	}
	ticker := time.NewTicker(cfg.ProgressEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s := recorder.Snapshot()
			logger.Info("progress",
				"operations", s.TotalOperations,
				"errors", s.TotalErrors,
				"ops_per_sec", fmt.Sprintf("%.2f", s.OperationsPerSecond),
				"p95_ms", fmt.Sprintf("%.1f", float64(s.P95)/float64(time.Millisecond)),
			)
		}
	}
}
