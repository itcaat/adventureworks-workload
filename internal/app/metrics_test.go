package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRecorderReportAggregatesOperationMetrics(t *testing.T) {
	cfg := Config{Users: 2, Profile: "mixed", WriteMode: "off"}
	rec := NewRecorder("test-run", cfg, nil)
	rec.Record("catalog_search", "read", "shopper", 10*time.Millisecond, nil, TrafficStats{Sent: 100, Received: 500})
	rec.Record("catalog_search", "read", "shopper", 20*time.Millisecond, errors.New("timeout"), TrafficStats{Sent: 100, Received: 200})
	rec.Record("sales_dashboard", "report", "analyst", 30*time.Millisecond, nil, TrafficStats{Sent: 80, Received: 4096})

	report := rec.Report(time.Now().Add(-time.Second), time.Now())
	if report.TotalOperations != 3 {
		t.Fatalf("TotalOperations = %d, want 3", report.TotalOperations)
	}
	if report.TotalErrors != 1 {
		t.Fatalf("TotalErrors = %d, want 1", report.TotalErrors)
	}
	if report.Personas["shopper"] != 2 || report.Personas["analyst"] != 1 {
		t.Fatalf("unexpected persona counts: %#v", report.Personas)
	}
	if len(report.Operations) != 2 {
		t.Fatalf("len(Operations) = %d, want 2", len(report.Operations))
	}
	if report.BytesSent != 280 {
		t.Fatalf("BytesSent = %d, want 280", report.BytesSent)
	}
	if report.BytesReceived != 4796 {
		t.Fatalf("BytesReceived = %d, want 4796", report.BytesReceived)
	}
}

func TestPercentile(t *testing.T) {
	samples := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}

	if got := percentile(samples, 0.50); got != 30*time.Millisecond {
		t.Fatalf("p50 = %s, want 30ms", got)
	}
	if got := percentile(samples, 0.95); got != 40*time.Millisecond {
		t.Fatalf("p95 = %s, want 40ms", got)
	}
	if got := percentile(nil, 0.95); got != 0 {
		t.Fatalf("empty percentile = %s, want 0", got)
	}
}

func TestNormalizeErrorTruncatesLongMessages(t *testing.T) {
	long := strings.Repeat("x", 300)
	got := normalizeError(errors.New(long))
	if len(got) <= 240 {
		t.Fatalf("expected truncated message, got len=%d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
}

func TestLiveSnapshotIncludesOperationsAndPersonas(t *testing.T) {
	cfg := Config{Users: 4, Ramp: 8 * time.Second, Duration: time.Minute, Profile: "mixed", WriteMode: "off"}
	rec := NewRecorder("test-run", cfg, nil)
	rec.Record("catalog_search", "read", "shopper", 10*time.Millisecond, nil, TrafficStats{Sent: 50, Received: 1000})
	rec.Record("sales_dashboard", "report", "analyst", 30*time.Millisecond, nil, TrafficStats{Sent: 40, Received: 8000})

	snap := rec.LiveSnapshot(2*time.Second, PhaseRamping, 2)
	if snap.RunID != "test-run" {
		t.Fatalf("RunID = %q, want test-run", snap.RunID)
	}
	if snap.Phase != PhaseRamping {
		t.Fatalf("Phase = %q, want ramping", snap.Phase)
	}
	if snap.ActiveUsers != 2 {
		t.Fatalf("ActiveUsers = %d, want 2", snap.ActiveUsers)
	}
	if len(snap.Operations) != 2 {
		t.Fatalf("len(Operations) = %d, want 2", len(snap.Operations))
	}
	if snap.Personas["shopper"] != 1 || snap.Personas["analyst"] != 1 {
		t.Fatalf("unexpected personas: %#v", snap.Personas)
	}
}

func TestComputePhase(t *testing.T) {
	cfg := Config{Ramp: time.Second, Duration: time.Minute}
	workloadCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Hour))
	defer cancel()

	if got := computePhase(500*time.Millisecond, cfg, workloadCtx); got != PhaseRamping {
		t.Fatalf("phase during ramp = %q, want ramping", got)
	}
	if got := computePhase(2*time.Second, cfg, workloadCtx); got != PhaseRunning {
		t.Fatalf("phase during run = %q, want running", got)
	}

	drainedCtx, drainCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	drainCancel()
	if got := computePhase(time.Minute, cfg, drainedCtx); got != PhaseDraining {
		t.Fatalf("phase after deadline = %q, want draining", got)
	}

	stoppedCtx, stopCancel := context.WithCancel(context.Background())
	stopCancel()
	if got := computePhase(time.Second, cfg, stoppedCtx); got != PhaseStopping {
		t.Fatalf("phase after cancel = %q, want stopping", got)
	}
}

func TestEstimatedRampUsers(t *testing.T) {
	cfg := Config{Users: 4, Ramp: 8 * time.Second}
	if got := estimatedRampUsers(cfg, 2*time.Second); got != 2 {
		t.Fatalf("estimatedRampUsers() = %d, want 2", got)
	}
	if got := estimatedRampUsers(cfg, 8*time.Second); got != 4 {
		t.Fatalf("estimatedRampUsers() at end of ramp = %d, want 4", got)
	}
}

func TestUserTrackerCountsActiveUsers(t *testing.T) {
	tracker := &userTracker{}
	if got := tracker.count(); got != 0 {
		t.Fatalf("initial count = %d, want 0", got)
	}
	tracker.add(1)
	tracker.add(1)
	if got := tracker.count(); got != 2 {
		t.Fatalf("count after add = %d, want 2", got)
	}
	tracker.add(-1)
	if got := tracker.count(); got != 1 {
		t.Fatalf("count after drain = %d, want 1", got)
	}
}

func TestReportProgressStopsAfterWorkload(t *testing.T) {
	rec := NewRecorder("test", Config{Users: 1}, nil)
	tracker := &userTracker{}
	workloadCtx, cancelWorkload := context.WithCancel(context.Background())
	defer cancelWorkload()

	stop := make(chan struct{})
	done := make(chan struct{})
	go reportProgress(context.Background(), workloadCtx, rec, tracker, Config{
		ProgressEvery: 10 * time.Millisecond,
	}, time.Now(), nil, nil, stop, done)

	close(stop)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reportProgress did not stop after workload completion signal")
	}
}

func TestSnapshotComputesThroughput(t *testing.T) {
	cfg := Config{Users: 1, Profile: "mixed", WriteMode: "off"}
	rec := NewRecorder("test-run", cfg, nil)
	rec.started = time.Now().Add(-2 * time.Second)
	rec.Record("catalog_search", "read", "shopper", 100*time.Millisecond, nil, TrafficStats{})
	rec.Record("catalog_search", "read", "shopper", 200*time.Millisecond, nil, TrafficStats{})

	snap := rec.Snapshot()
	if snap.TotalOperations != 2 {
		t.Fatalf("TotalOperations = %d, want 2", snap.TotalOperations)
	}
	if snap.OperationsPerSecond <= 0 {
		t.Fatalf("OperationsPerSecond = %f, want > 0", snap.OperationsPerSecond)
	}
}

func TestConfigRejectsUnsafeMissingCredentials(t *testing.T) {
	_, err := ParseConfig([]string{"-duration", "1s"}, func(string) string { return "" })
	if err == nil {
		t.Fatal("expected validation error")
	}
}
