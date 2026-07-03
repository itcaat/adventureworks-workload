package tui

import (
	"fmt"
	"sort"
	"strings"

	"adventureworks-workload/internal/app"
)

func renderReportContent(cfg app.Config, report app.Report, mdPath, jsonPath string) string {
	if report.RunID == "" {
		return labelStyle.Render("Run stopped before a report was generated.")
	}

	errPct := 0.0
	if report.TotalOperations > 0 {
		errPct = float64(report.TotalErrors) / float64(report.TotalOperations) * 100
	}

	meta := fmt.Sprintf(
		"%s · write %s · %d users · elapsed %s",
		report.Config.Profile,
		report.Config.WriteMode,
		report.Config.Users,
		formatDuration(report.Elapsed),
	)

	summary := strings.Join([]string{
		fmt.Sprintf("%s %s", labelStyle.Render("throughput"), valueStyle.Render(fmt.Sprintf("%.2f ops/s", report.OperationsPerSecond))),
		fmt.Sprintf("%s %s", labelStyle.Render("operations"), valueStyle.Render(fmt.Sprintf("%d", report.TotalOperations))),
		fmt.Sprintf("%s %s", labelStyle.Render("errors"), valueStyle.Render(fmt.Sprintf("%d (%.1f%%)", report.TotalErrors, errPct))),
		fmt.Sprintf("%s %s", labelStyle.Render("p50"), valueStyle.Render(formatDuration(report.P50))),
		fmt.Sprintf("%s %s", labelStyle.Render("p95"), valueStyle.Render(formatDuration(report.P95))),
		fmt.Sprintf("%s %s", labelStyle.Render("p99"), valueStyle.Render(formatDuration(report.P99))),
	}, "   ")

	traffic := fmt.Sprintf("%s %s   %s %s",
		labelStyle.Render("↑ sent"), valueStyle.Render(fmt.Sprintf("%s (%.2f MB/s)", app.FormatBytes(report.BytesSent), report.BytesSentPerSecond/(1<<20))),
		labelStyle.Render("↓ recv"), valueStyle.Render(fmt.Sprintf("%s (%.2f MB/s)", app.FormatBytes(report.BytesReceived), report.BytesReceivedPerSecond/(1<<20))),
	)

	timelineChart := renderTimelineChart(report.Timeline)
	ops := renderFinalOperationsTable(report.Operations)
	personas := renderPersonas(report.Personas)
	errors := renderErrorSamples(report)

	sections := []string{
		labelStyle.Render(meta),
		"",
		summary,
		traffic,
		"",
		labelStyle.Render("Requests over time"),
		timelineChart,
		"",
		labelStyle.Render("Operations"),
		ops,
		"",
		labelStyle.Render("Personas"),
		personas,
	}
	if errors != "" {
		sections = append(sections, "", labelStyle.Render("Errors"), errors)
	}
	if mdPath != "" || jsonPath != "" {
		sections = append(sections, "",
			labelStyle.Render("Saved reports"),
			valueStyle.Render("  "+mdPath),
			valueStyle.Render("  "+jsonPath),
		)
	}
	_ = cfg
	return strings.Join(sections, "\n")
}

func renderFinalOperationsTable(ops []app.OperationReport) string {
	if len(ops) == 0 {
		return labelStyle.Render("  —")
	}

	live := make([]app.OperationLive, len(ops))
	for i, op := range ops {
		live[i] = app.OperationLive{
			Name:          op.Name,
			Kind:          op.Kind,
			Count:         op.Count,
			Errors:        op.Errors,
			ErrorRate:     op.ErrorRate,
			P95:           op.P95,
			BytesSent:     op.BytesSent,
			BytesReceived: op.BytesReceived,
		}
	}
	return renderOperationsTable(live)
}

func renderErrorSamples(report app.Report) string {
	type sample struct {
		operation string
		message   string
		count     int64
	}
	var samples []sample
	for _, op := range report.Operations {
		for msg, count := range op.Failures {
			samples = append(samples, sample{operation: op.Name, message: msg, count: count})
		}
	}
	if len(samples) == 0 {
		return ""
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].count > samples[j].count })
	if len(samples) > 8 {
		samples = samples[:8]
	}
	lines := make([]string, 0, len(samples))
	for _, s := range samples {
		lines = append(lines, valueStyle.Render(fmt.Sprintf("  %s: %d x %s", s.operation, s.count, s.message)))
	}
	return strings.Join(lines, "\n")
}
