package app

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRecorderReportAggregatesOperationMetrics(t *testing.T) {
	cfg := Config{Users: 2, Profile: "mixed", WriteMode: "off"}
	rec := NewRecorder("test-run", cfg, nil)
	rec.Record("catalog_search", "read", "shopper", 10*time.Millisecond, nil)
	rec.Record("catalog_search", "read", "shopper", 20*time.Millisecond, errors.New("timeout"))
	rec.Record("sales_dashboard", "report", "analyst", 30*time.Millisecond, nil)

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

func TestSnapshotComputesThroughput(t *testing.T) {
	cfg := Config{Users: 1, Profile: "mixed", WriteMode: "off"}
	rec := NewRecorder("test-run", cfg, nil)
	rec.started = time.Now().Add(-2 * time.Second)
	rec.Record("catalog_search", "read", "shopper", 100*time.Millisecond, nil)
	rec.Record("catalog_search", "read", "shopper", 200*time.Millisecond, nil)

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
