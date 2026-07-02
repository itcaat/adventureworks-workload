package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Report struct {
	RunID               string            `json:"run_id"`
	StartedAt           time.Time         `json:"started_at"`
	EndedAt             time.Time         `json:"ended_at"`
	GeneratedAt         time.Time         `json:"generated_at"`
	Elapsed             time.Duration     `json:"elapsed"`
	Config              Config            `json:"-"`
	TotalOperations     int64             `json:"total_operations"`
	TotalErrors         int64             `json:"total_errors"`
	OperationsPerSecond float64           `json:"operations_per_second"`
	P50                 time.Duration     `json:"p50"`
	P95                 time.Duration     `json:"p95"`
	P99                 time.Duration     `json:"p99"`
	BytesSent           int64             `json:"bytes_sent"`
	BytesReceived       int64             `json:"bytes_received"`
	BytesSentPerSecond     float64        `json:"bytes_sent_per_second"`
	BytesReceivedPerSecond float64        `json:"bytes_received_per_second"`
	Operations          []OperationReport `json:"operations"`
	Personas            map[string]int64  `json:"personas"`
}

type OperationReport struct {
	Name       string           `json:"name"`
	Kind       string           `json:"kind"`
	Count      int64            `json:"count"`
	Errors     int64            `json:"errors"`
	ErrorRate  float64          `json:"error_rate"`
	Avg        time.Duration    `json:"avg"`
	Min        time.Duration    `json:"min"`
	Max        time.Duration    `json:"max"`
	P50        time.Duration    `json:"p50"`
	P95        time.Duration    `json:"p95"`
	P99            time.Duration    `json:"p99"`
	SampleSize     int              `json:"sample_size"`
	Failures       map[string]int64 `json:"failures,omitempty"`
	BytesSent      int64            `json:"bytes_sent"`
	BytesReceived  int64            `json:"bytes_received"`
	AvgBytesSent   int64            `json:"avg_bytes_sent"`
	AvgBytesReceived int64          `json:"avg_bytes_received"`
}

func reportBaseName(cfg Config, runID string) string {
	if cfg.ReportName != "" {
		return cfg.ReportName + "-" + runID
	}
	return "awload-" + runID
}

func WriteReports(report Report, cfg Config) error {
	if err := os.MkdirAll(cfg.ReportDir, 0o755); err != nil {
		return err
	}
	base := reportBaseName(cfg, report.RunID)
	mdPath := filepath.Join(cfg.ReportDir, base+".md")
	jsonPath := filepath.Join(cfg.ReportDir, base+".json")

	if err := os.WriteFile(mdPath, []byte(report.Markdown()), 0o644); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(report.jsonView(), "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, encoded, 0o644); err != nil {
		return err
	}
	fmt.Printf("reports written: %s, %s\n", mdPath, jsonPath)
	return nil
}

func (r Report) MarkdownSummary() string {
	return fmt.Sprintf("Run %s: %d operations, %d errors, %.2f ops/sec, p95 %s, sent %s (%.1f/s), recv %s (%.1f/s)",
		r.RunID,
		r.TotalOperations,
		r.TotalErrors,
		r.OperationsPerSecond,
		roundDuration(r.P95),
		FormatBytes(r.BytesSent),
		r.BytesSentPerSecond/(1<<20),
		FormatBytes(r.BytesReceived),
		r.BytesReceivedPerSecond/(1<<20),
	)
}

func (r Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# AdventureWorks Workload Report\n\n")
	fmt.Fprintf(&b, "- Run ID: `%s`\n", r.RunID)
	fmt.Fprintf(&b, "- Started: `%s`\n", r.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- Ended: `%s`\n", r.EndedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- Elapsed: `%s`\n", roundDuration(r.Elapsed))
	fmt.Fprintf(&b, "- Users: `%d`\n", r.Config.Users)
	fmt.Fprintf(&b, "- Profile: `%s`\n", r.Config.Profile)
	fmt.Fprintf(&b, "- Write mode: `%s`\n", r.Config.WriteMode)
	fmt.Fprintf(&b, "- Total operations: `%d`\n", r.TotalOperations)
	fmt.Fprintf(&b, "- Errors: `%d`\n", r.TotalErrors)
	fmt.Fprintf(&b, "- Throughput: `%.2f ops/sec`\n", r.OperationsPerSecond)
	fmt.Fprintf(&b, "- Latency p50/p95/p99: `%s / %s / %s`\n", roundDuration(r.P50), roundDuration(r.P95), roundDuration(r.P99))
	fmt.Fprintf(&b, "- Payload sent: `%s` (`%.2f MB/s`)\n", FormatBytes(r.BytesSent), r.BytesSentPerSecond/(1<<20))
	fmt.Fprintf(&b, "- Payload received: `%s` (`%.2f MB/s`)\n\n", FormatBytes(r.BytesReceived), r.BytesReceivedPerSecond/(1<<20))

	fmt.Fprintf(&b, "## Operation Metrics\n\n")
	fmt.Fprintf(&b, "| Operation | Kind | Count | Errors | Error %% | Avg | P50 | P95 | P99 | Max | Sent | Recv | Avg recv |\n")
	fmt.Fprintf(&b, "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, op := range r.Operations {
		fmt.Fprintf(&b, "| `%s` | `%s` | %d | %d | %.2f | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			op.Name,
			op.Kind,
			op.Count,
			op.Errors,
			op.ErrorRate*100,
			roundDuration(op.Avg),
			roundDuration(op.P50),
			roundDuration(op.P95),
			roundDuration(op.P99),
			roundDuration(op.Max),
			FormatBytes(op.BytesSent),
			FormatBytes(op.BytesReceived),
			FormatBytes(op.AvgBytesReceived),
		)
	}

	fmt.Fprintf(&b, "\n## Persona Mix\n\n")
	personas := make([]string, 0, len(r.Personas))
	for name := range r.Personas {
		personas = append(personas, name)
	}
	sort.Strings(personas)
	for _, name := range personas {
		fmt.Fprintf(&b, "- `%s`: %d operations\n", name, r.Personas[name])
	}

	failures := r.failures()
	if len(failures) > 0 {
		fmt.Fprintf(&b, "\n## Error Samples\n\n")
		for _, f := range failures {
			fmt.Fprintf(&b, "- `%s`: %d x %s\n", f.Operation, f.Count, f.Message)
		}
	}

	return b.String()
}

type failureSample struct {
	Operation string
	Message   string
	Count     int64
}

func (r Report) failures() []failureSample {
	var out []failureSample
	for _, op := range r.Operations {
		for msg, count := range op.Failures {
			out = append(out, failureSample{Operation: op.Name, Message: msg, Count: count})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if len(out) > 10 {
		return out[:10]
	}
	return out
}

type jsonReport struct {
	RunID               string                 `json:"run_id"`
	StartedAt           time.Time              `json:"started_at"`
	EndedAt             time.Time              `json:"ended_at"`
	ElapsedMS           int64                  `json:"elapsed_ms"`
	TotalOperations        int64                  `json:"total_operations"`
	TotalErrors            int64                  `json:"total_errors"`
	OperationsPerSecond    float64                `json:"operations_per_second"`
	BytesSent              int64                  `json:"bytes_sent"`
	BytesReceived          int64                  `json:"bytes_received"`
	BytesSentPerSecond     float64                `json:"bytes_sent_per_second"`
	BytesReceivedPerSecond float64                `json:"bytes_received_per_second"`
	LatencyMS              map[string]float64     `json:"latency_ms"`
	Personas            map[string]int64       `json:"personas"`
	Operations          []jsonOperationReport  `json:"operations"`
	Config              map[string]interface{} `json:"config"`
}

type jsonOperationReport struct {
	Name       string             `json:"name"`
	Kind       string             `json:"kind"`
	Count      int64              `json:"count"`
	Errors     int64              `json:"errors"`
	ErrorRate          float64            `json:"error_rate"`
	LatencyMS          map[string]float64 `json:"latency_ms"`
	SampleSize         int                `json:"sample_size"`
	Failures           map[string]int64   `json:"failures,omitempty"`
	BytesSent          int64              `json:"bytes_sent"`
	BytesReceived      int64              `json:"bytes_received"`
	AvgBytesSent       int64              `json:"avg_bytes_sent"`
	AvgBytesReceived   int64              `json:"avg_bytes_received"`
}

func (r Report) jsonView() jsonReport {
	ops := make([]jsonOperationReport, 0, len(r.Operations))
	for _, op := range r.Operations {
		ops = append(ops, jsonOperationReport{
			Name:      op.Name,
			Kind:      op.Kind,
			Count:     op.Count,
			Errors:    op.Errors,
			ErrorRate: op.ErrorRate,
			LatencyMS: map[string]float64{
				"avg": durationMS(op.Avg),
				"min": durationMS(op.Min),
				"max": durationMS(op.Max),
				"p50": durationMS(op.P50),
				"p95": durationMS(op.P95),
				"p99": durationMS(op.P99),
			},
			SampleSize:       op.SampleSize,
			Failures:         op.Failures,
			BytesSent:        op.BytesSent,
			BytesReceived:    op.BytesReceived,
			AvgBytesSent:     op.AvgBytesSent,
			AvgBytesReceived: op.AvgBytesReceived,
		})
	}
	return jsonReport{
		RunID:                  r.RunID,
		StartedAt:              r.StartedAt,
		EndedAt:                r.EndedAt,
		ElapsedMS:              r.Elapsed.Milliseconds(),
		TotalOperations:        r.TotalOperations,
		TotalErrors:            r.TotalErrors,
		OperationsPerSecond:    r.OperationsPerSecond,
		BytesSent:              r.BytesSent,
		BytesReceived:          r.BytesReceived,
		BytesSentPerSecond:     r.BytesSentPerSecond,
		BytesReceivedPerSecond: r.BytesReceivedPerSecond,
		LatencyMS: map[string]float64{
			"p50": durationMS(r.P50),
			"p95": durationMS(r.P95),
			"p99": durationMS(r.P99),
		},
		Personas:   r.Personas,
		Operations: ops,
		Config: map[string]interface{}{
			"users":           r.Config.Users,
			"duration":        r.Config.Duration.String(),
			"ramp":            r.Config.Ramp.String(),
			"profile":         r.Config.Profile,
			"write_mode":      r.Config.WriteMode,
			"think_min":       r.Config.ThinkMin.String(),
			"think_max":       r.Config.ThinkMax.String(),
			"request_timeout": r.Config.RequestTimeout.String(),
		},
	}
}

func durationMS(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func roundDuration(d time.Duration) string {
	if d >= time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(100 * time.Microsecond).String()
}
