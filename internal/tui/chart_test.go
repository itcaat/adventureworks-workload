package tui

import (
	"strings"
	"testing"
	"time"

	"adventureworks-workload/internal/app"
)

func fixedTimelineBuckets() []app.TimelineBucket {
	buckets := make([]app.TimelineBucket, 40)
	for i := range buckets {
		start := time.Duration(i) * (60 * time.Second / 40)
		buckets[i] = app.TimelineBucket{
			Start:      start,
			End:        start + (60 * time.Second / 40),
			Operations: 0,
			Errors:     0,
		}
	}
	buckets[10] = app.TimelineBucket{Start: buckets[10].Start, End: buckets[10].End, Operations: 50, Errors: 10}
	buckets[20] = app.TimelineBucket{Start: buckets[20].Start, End: buckets[20].End, Operations: 30, Errors: 30}
	return buckets
}

func TestRenderTimelineChartShowsStackedBars(t *testing.T) {
	view := renderTimelineChart(fixedTimelineBuckets())
	if !strings.Contains(view, "█") {
		t.Fatalf("expected chart bars, got:\n%s", view)
	}
	if !strings.Contains(view, "ok") || !strings.Contains(view, "errors") {
		t.Fatalf("expected legend, got:\n%s", view)
	}
}

func TestRenderTimelineChartKeepsFullWidthBeforeData(t *testing.T) {
	buckets := make([]app.TimelineBucket, 40)
	for i := range buckets {
		start := time.Duration(i) * time.Second
		buckets[i] = app.TimelineBucket{Start: start, End: start + time.Second}
	}
	view := renderTimelineChart(buckets)
	barWidth := strings.Count(view, "░") + strings.Count(view, "█")
	if barWidth < 40*8 {
		t.Fatalf("expected full-width empty chart, got:\n%s", view)
	}
}

func TestTimelineCellAllGreenWhenNoErrors(t *testing.T) {
	bucket := app.TimelineBucket{Operations: 100, Errors: 0}
	if cell := timelineCell(bucket, 7, 8, 100); cell == "" {
		t.Fatal("expected filled bottom cell")
	}
}
