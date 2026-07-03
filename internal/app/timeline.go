package app

import (
	"math"
	"time"
)

const targetTimelineBuckets = 40

type TimelineBucket struct {
	Start      time.Duration `json:"start"`
	End        time.Duration `json:"end"`
	Operations int64         `json:"operations"`
	Errors     int64         `json:"errors"`
}

func (b TimelineBucket) Successes() int64 {
	return b.Operations - b.Errors
}

type timelineSlot struct {
	operations int64
	errors     int64
}

func timelineBucketSize(window time.Duration) time.Duration {
	if window <= 0 {
		return time.Second
	}
	secs := window.Seconds()
	if secs <= float64(targetTimelineBuckets) {
		return time.Second
	}
	bucket := int(math.Ceil(secs / float64(targetTimelineBuckets)))
	if bucket < 1 {
		bucket = 1
	}
	return time.Duration(bucket) * time.Second
}

func timelineBucketCount(window, bucketSize time.Duration) int {
	if window <= 0 || bucketSize <= 0 {
		return targetTimelineBuckets
	}
	n := int(window / bucketSize)
	if window%bucketSize != 0 {
		n++
	}
	return max(1, n)
}

func (r *Recorder) recordTimelineBucket(err error) {
	idx := int(time.Since(r.started) / r.timelineBucketSize)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(r.timelineBuckets) {
		needed := idx + 1
		grown := make([]timelineSlot, needed)
		copy(grown, r.timelineBuckets)
		r.timelineBuckets = grown
	}
	r.timelineBuckets[idx].operations++
	if err != nil {
		r.timelineBuckets[idx].errors++
	}
}

func (r *Recorder) buildTimeline() []TimelineBucket {
	out := make([]TimelineBucket, 0, len(r.timelineBuckets))
	for i, slot := range r.timelineBuckets {
		start := time.Duration(i) * r.timelineBucketSize
		end := start + r.timelineBucketSize
		out = append(out, TimelineBucket{
			Start:      start,
			End:        end,
			Operations: slot.operations,
			Errors:     slot.errors,
		})
	}
	return out
}
