package tui

import (
	"strings"
	"testing"
	"time"

	"adventureworks-workload/internal/app"
)

func TestRenderReportContentShowsSummaryAndPaths(t *testing.T) {
	report := app.Report{
		RunID:               "20260702T133255Z",
		Elapsed:             time.Minute,
		Config:              app.Config{Users: 100, Profile: "write-light", WriteMode: "cart"},
		TotalOperations:     1200,
		TotalErrors:         3,
		OperationsPerSecond: 20,
		P95:                 200 * time.Millisecond,
		BytesSent:           400 << 10,
		BytesReceived:       700 << 10,
		Operations: []app.OperationReport{{
			Name:  "catalog_search",
			Kind:  "read",
			Count: 100,
			P95:   150 * time.Millisecond,
		}},
		Personas: map[string]int64{"shopper": 100},
	}

	view := renderReportContent(app.Config{}, report, "reports/smoke.md", "reports/smoke.json")
	for _, want := range []string{
		"write-light",
		"catalog_search",
		"shopper 100",
		"reports/smoke.md",
		"reports/smoke.json",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderReportContent() missing %q\n%s", want, view)
		}
	}
}
