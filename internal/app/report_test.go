package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReportBaseName(t *testing.T) {
	runID := "20260702T123530Z"

	t.Run("custom name", func(t *testing.T) {
		got := reportBaseName(Config{ReportName: "smoke-write"}, runID)
		want := "smoke-write-20260702T123530Z"
		if got != want {
			t.Fatalf("reportBaseName() = %q, want %q", got, want)
		}
	})

	t.Run("default name", func(t *testing.T) {
		got := reportBaseName(Config{}, runID)
		want := "awload-20260702T123530Z"
		if got != want {
			t.Fatalf("reportBaseName() = %q, want %q", got, want)
		}
	})
}

func TestReportMarkdownIncludesKeySections(t *testing.T) {
	report := Report{
		RunID:               "20260702T123530Z",
		StartedAt:           time.Date(2026, 7, 2, 12, 35, 30, 0, time.UTC),
		EndedAt:             time.Date(2026, 7, 2, 12, 36, 30, 0, time.UTC),
		Elapsed:             time.Minute,
		Config:              Config{Users: 10, Profile: "mixed", WriteMode: "off"},
		TotalOperations:     100,
		TotalErrors:         2,
		OperationsPerSecond: 1.67,
		Operations: []OperationReport{
			{
				Name:      "catalog_search",
				Kind:      "read",
				Count:     100,
				Errors:    2,
				ErrorRate: 0.02,
				Avg:       10 * time.Millisecond,
				P50:       10 * time.Millisecond,
				P95:       20 * time.Millisecond,
				P99:       30 * time.Millisecond,
				Max:       40 * time.Millisecond,
				Failures:  map[string]int64{"context deadline exceeded": 2},
			},
		},
		Personas: map[string]int64{"shopper": 100},
	}

	md := report.Markdown()
	for _, want := range []string{
		"# AdventureWorks Workload Report",
		"## Operation Metrics",
		"## Persona Mix",
		"## Error Samples",
		"`catalog_search`",
		"context deadline exceeded",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("Markdown missing %q\n%s", want, md)
		}
	}
}

func TestWriteReportsCreatesTimestampedFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{ReportDir: dir, ReportName: "smoke-write"}
	report := Report{
		RunID:     "20260702T123530Z",
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
		Config:    cfg,
		Personas:  map[string]int64{},
	}

	if err := WriteReports(report, cfg); err != nil {
		t.Fatalf("WriteReports() error = %v", err)
	}

	mdPath := filepath.Join(dir, "smoke-write-20260702T123530Z.md")
	jsonPath := filepath.Join(dir, "smoke-write-20260702T123530Z.json")
	for _, path := range []string{mdPath, jsonPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected report file %q: %v", path, err)
		}
	}
}
