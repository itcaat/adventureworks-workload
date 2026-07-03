package app

import (
	"sort"
	"strings"
	"sync"
	"time"
)

const maxLatencySamplesPerOperation = 200000

type Recorder struct {
	mu              sync.Mutex
	runID           string
	cfg             Config
	ops             []Operation
	started         time.Time
	timelineBucketSize time.Duration
	timelineBuckets    []timelineSlot
	stats           map[string]*operationRecorder
	persona         map[string]int64
}

type operationRecorder struct {
	Name          string
	Kind          string
	Count         int64
	Errors        int64
	Total         time.Duration
	Min           time.Duration
	Max           time.Duration
	Latencies     []time.Duration
	Failures      map[string]int64
	BytesSent     int64
	BytesReceived int64
}

type Snapshot struct {
	TotalOperations     int64
	TotalErrors         int64
	OperationsPerSecond float64
	P95                 time.Duration
}

func NewRecorder(runID string, cfg Config, ops []Operation) *Recorder {
	timelineBucketSize := timelineBucketSize(cfg.Duration)
	timelineBucketCount := timelineBucketCount(cfg.Duration, timelineBucketSize)
	return &Recorder{
		runID:              runID,
		cfg:                cfg,
		ops:                ops,
		started:            time.Now(),
		timelineBucketSize: timelineBucketSize,
		timelineBuckets:    make([]timelineSlot, timelineBucketCount),
		stats:              map[string]*operationRecorder{},
		persona:            map[string]int64{},
	}
}

func (r *Recorder) Record(name, kind, persona string, latency time.Duration, err error, traffic TrafficStats) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s := r.stats[name]
	if s == nil {
		s = &operationRecorder{Name: name, Kind: kind, Failures: map[string]int64{}}
		r.stats[name] = s
	}
	s.Count++
	s.Total += latency
	s.BytesSent += traffic.Sent
	s.BytesReceived += traffic.Received
	if s.Min == 0 || latency < s.Min {
		s.Min = latency
	}
	if latency > s.Max {
		s.Max = latency
	}
	if len(s.Latencies) < maxLatencySamplesPerOperation {
		s.Latencies = append(s.Latencies, latency)
	}
	if err != nil {
		s.Errors++
		s.Failures[normalizeError(err)]++
	}
	r.recordTimelineBucket(err)
	r.persona[persona]++
}

func (r *Recorder) Snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked()
}

func (r *Recorder) LiveSnapshot(elapsed time.Duration, phase RunPhase, activeUsers int) LiveSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	base := r.snapshotLocked()
	ops := make([]OperationLive, 0, len(r.stats))
	for _, s := range r.stats {
		live := OperationLive{
			Name:          s.Name,
			Kind:          s.Kind,
			Count:         s.Count,
			Errors:        s.Errors,
			P95:           percentile(s.Latencies, 0.95),
			BytesSent:     s.BytesSent,
			BytesReceived: s.BytesReceived,
		}
		if s.Count > 0 {
			live.ErrorRate = float64(s.Errors) / float64(s.Count)
		}
		ops = append(ops, live)
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].Name < ops[j].Name })

	personas := make(map[string]int64, len(r.persona))
	for k, v := range r.persona {
		personas[k] = v
	}

	var bytesSent, bytesReceived int64
	for _, s := range r.stats {
		bytesSent += s.BytesSent
		bytesReceived += s.BytesReceived
	}
	elapsedSec := elapsed.Seconds()
	if elapsedSec == 0 {
		elapsedSec = 1
	}

	return LiveSnapshot{
		RunID:               r.runID,
		Profile:             r.cfg.Profile,
		WriteMode:           r.cfg.WriteMode,
		Users:               r.cfg.Users,
		Duration:            r.cfg.Duration,
		Ramp:                r.cfg.Ramp,
		Elapsed:             elapsed,
		Phase:                  phase,
		ActiveUsers:            activeUsers,
		TotalOperations:     base.TotalOperations,
		TotalErrors:         base.TotalErrors,
		OperationsPerSecond: base.OperationsPerSecond,
		P50:                 percentile(allLatencies(r.stats), 0.50),
		P95:                 base.P95,
		P99:                 percentile(allLatencies(r.stats), 0.99),
		BytesSent:           bytesSent,
		BytesReceived:       bytesReceived,
		BytesSentPerSecond:  float64(bytesSent) / elapsedSec,
		BytesReceivedPerSecond: float64(bytesReceived) / elapsedSec,
		Timeline:               r.buildTimeline(),
		Operations:             ops,
		Personas:            personas,
	}
}

func (r *Recorder) snapshotLocked() Snapshot {
	var all []time.Duration
	var total, errors int64
	for _, s := range r.stats {
		total += s.Count
		errors += s.Errors
		all = append(all, s.Latencies...)
	}
	elapsed := time.Since(r.started).Seconds()
	if elapsed == 0 {
		elapsed = 1
	}
	return Snapshot{
		TotalOperations:     total,
		TotalErrors:         errors,
		OperationsPerSecond: float64(total) / elapsed,
		P95:                 percentile(all, 0.95),
	}
}

func allLatencies(stats map[string]*operationRecorder) []time.Duration {
	var all []time.Duration
	for _, s := range stats {
		all = append(all, s.Latencies...)
	}
	return all
}

func (r *Recorder) Report(started, ended time.Time) Report {
	r.mu.Lock()
	defer r.mu.Unlock()

	report := Report{
		RunID:       r.runID,
		StartedAt:   started,
		EndedAt:     ended,
		Elapsed:     ended.Sub(started),
		Config:      r.cfg,
		Operations:  make([]OperationReport, 0, len(r.stats)),
		Personas:    make(map[string]int64, len(r.persona)),
		GeneratedAt: time.Now(),
	}

	var all []time.Duration
	for _, s := range r.stats {
		report.TotalOperations += s.Count
		report.TotalErrors += s.Errors
		report.BytesSent += s.BytesSent
		report.BytesReceived += s.BytesReceived
		all = append(all, s.Latencies...)
		report.Operations = append(report.Operations, operationReport(*s))
	}
	sort.Slice(report.Operations, func(i, j int) bool {
		return report.Operations[i].Name < report.Operations[j].Name
	})
	for k, v := range r.persona {
		report.Personas[k] = v
	}
	if report.Elapsed > 0 {
		secs := report.Elapsed.Seconds()
		report.OperationsPerSecond = float64(report.TotalOperations) / secs
		report.BytesSentPerSecond = float64(report.BytesSent) / secs
		report.BytesReceivedPerSecond = float64(report.BytesReceived) / secs
	}
	report.P50 = percentile(all, 0.50)
	report.P95 = percentile(all, 0.95)
	report.P99 = percentile(all, 0.99)
	report.Timeline = r.buildTimeline()
	return report
}

func operationReport(s operationRecorder) OperationReport {
	out := OperationReport{
		Name:          s.Name,
		Kind:          s.Kind,
		Count:         s.Count,
		Errors:        s.Errors,
		Min:           s.Min,
		Max:           s.Max,
		P50:           percentile(s.Latencies, 0.50),
		P95:           percentile(s.Latencies, 0.95),
		P99:           percentile(s.Latencies, 0.99),
		Avg:           0,
		Failures:      s.Failures,
		SampleSize:    len(s.Latencies),
		BytesSent:     s.BytesSent,
		BytesReceived: s.BytesReceived,
	}
	if s.Count > 0 {
		out.Avg = time.Duration(int64(s.Total) / s.Count)
		out.AvgBytesSent = s.BytesSent / s.Count
		out.AvgBytesReceived = s.BytesReceived / s.Count
	}
	if s.Count > 0 {
		out.ErrorRate = float64(s.Errors) / float64(s.Count)
	}
	return out
}

func percentile(samples []time.Duration, p float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	cp := append([]time.Duration(nil), samples...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	idx := int(float64(len(cp)-1) * p)
	return cp[idx]
}

func normalizeError(err error) string {
	msg := strings.ReplaceAll(err.Error(), "\n", " ")
	if len(msg) > 240 {
		return msg[:240] + "..."
	}
	return msg
}
