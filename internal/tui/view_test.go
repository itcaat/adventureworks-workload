package tui

import (
	"strings"
	"testing"
	"time"

	"adventureworks-workload/internal/app"
)

func TestRenderDashboardShowsRunMetadata(t *testing.T) {
	cfg := app.Config{
		ReportDir: "reports",
	}
	snap := app.LiveSnapshot{
		RunID:               "20260702T123530Z",
		Profile:             "read-heavy",
		WriteMode:           "off",
		Users:               10,
		Duration:            time.Minute,
		Ramp:                5 * time.Second,
		Elapsed:             30 * time.Second,
		Phase:               app.PhaseRunning,
		ActiveUsers:         10,
		TotalOperations:     1200,
		TotalErrors:         3,
		OperationsPerSecond: 40,
		BytesSent:           120 << 10,
		BytesReceived:       48 << 20,
		BytesSentPerSecond:  4 << 10,
		BytesReceivedPerSecond: 1.6 * (1 << 20),
		P50:                 100 * time.Millisecond,
		P95:                 250 * time.Millisecond,
		P99:                 400 * time.Millisecond,
		Operations: []app.OperationLive{{
			Name:          "catalog_search",
			Kind:          "read",
			Count:         500,
			Errors:        1,
			ErrorRate:     0.002,
			P95:           200 * time.Millisecond,
			BytesSent:     500 * 400,
			BytesReceived: 500 * 48000,
		}},
		Personas: map[string]int64{"shopper": 600, "support": 600},
	}

	view := renderDashboard(cfg, snap)
	for _, want := range []string{
		"20260702T123530Z",
		"read-heavy",
		"catalog_search",
		"shopper 600",
		"reports",
		"sent",
		"recv",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderDashboard() missing %q\n%s", want, view)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	if got := formatDuration(0); got != "—" {
		t.Fatalf("formatDuration(0) = %q, want dash", got)
	}
	if got := formatDuration(1500 * time.Microsecond); got == "—" {
		t.Fatalf("formatDuration(1500µs) = %q, want duration", got)
	}
}
