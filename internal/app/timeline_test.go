package app

import (
	"strings"
	"testing"
	"time"
)

func TestBuildTimelineUsesFixedDurationWidth(t *testing.T) {
	rec := NewRecorder("run", Config{Duration: 60 * time.Second}, nil)
	if len(rec.timelineBuckets) == 0 {
		t.Fatal("expected preallocated timeline buckets")
	}

	rec.timelineBuckets[2] = timelineSlot{operations: 9, errors: 8}

	timeline := rec.buildTimeline()
	if len(timeline) != len(rec.timelineBuckets) {
		t.Fatalf("len(timeline) = %d, want %d", len(timeline), len(rec.timelineBuckets))
	}
	if timeline[2].Operations != 9 || timeline[2].Errors != 8 {
		t.Fatalf("bucket[2] = %+v, want 9 ops / 8 errors", timeline[2])
	}
	if timeline[len(timeline)-1].End != 60*time.Second {
		t.Fatalf("last bucket end = %s, want 60s", timeline[len(timeline)-1].End)
	}
}

func TestRecordTimelineBucketTracksAllRequests(t *testing.T) {
	rec := NewRecorder("run", Config{Duration: time.Minute}, nil)
	rec.timelineBucketSize = time.Second
	rec.timelineBuckets = make([]timelineSlot, 60)

	rec.recordTimelineBucket(nil)
	rec.recordTimelineBucket(nil)
	rec.recordTimelineBucket(fmtErr("boom"))

	if rec.timelineBuckets[0].operations != 3 {
		t.Fatalf("operations = %d, want 3", rec.timelineBuckets[0].operations)
	}
	if rec.timelineBuckets[0].errors != 1 {
		t.Fatalf("errors = %d, want 1", rec.timelineBuckets[0].errors)
	}
}

func TestTimelineBucketCountForDuration(t *testing.T) {
	size := timelineBucketSize(60 * time.Second)
	count := timelineBucketCount(60*time.Second, size)
	if count < 30 || count > 60 {
		t.Fatalf("bucket count = %d, want roughly 40 for 60s window", count)
	}
}

func TestRecordTimelineBucketGrowsDuringDrain(t *testing.T) {
	rec := NewRecorder("run", Config{Duration: 60 * time.Second}, nil)
	initialLen := len(rec.timelineBuckets)
	rec.started = time.Now().Add(-65 * time.Second)

	rec.recordTimelineBucket(nil)

	if len(rec.timelineBuckets) <= initialLen {
		t.Fatalf("len(timelineBuckets) = %d, want > %d", len(rec.timelineBuckets), initialLen)
	}
	last := rec.timelineBuckets[len(rec.timelineBuckets)-1]
	if last.operations != 1 {
		t.Fatalf("last bucket operations = %d, want 1", last.operations)
	}

	timeline := rec.buildTimeline()
	if timeline[len(timeline)-1].End <= 60*time.Second {
		t.Fatalf("last bucket end = %s, want > 60s", timeline[len(timeline)-1].End)
	}
}

func TestMarkdownOmitsTimelineChartSection(t *testing.T) {
	report := Report{
		RunID:     "run",
		StartedAt: time.Now(),
		EndedAt:   time.Now().Add(time.Minute),
		Elapsed:   time.Minute,
		Config:    Config{Users: 1, Profile: "mixed", WriteMode: "off"},
		Timeline:  []TimelineBucket{{Start: 0, End: time.Minute, Operations: 10, Errors: 2}},
		Personas:  map[string]int64{},
	}
	if strings.Contains(report.Markdown(), "## Errors Over Time") {
		t.Fatal("markdown report should not include timeline chart section")
	}
}

type fmtErr string

func (e fmtErr) Error() string { return string(e) }
