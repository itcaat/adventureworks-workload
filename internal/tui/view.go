package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"adventureworks-workload/internal/app"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func renderDashboard(cfg app.Config, snap app.LiveSnapshot) string {
	runID := snap.RunID
	if runID == "" {
		runID = cfg.RunID()
	}

	header := titleStyle.Render(fmt.Sprintf(" awload — %s ", runID))
	meta := fmt.Sprintf(
		"%s · write %s · %d users · ramp %s",
		snap.Profile,
		snap.WriteMode,
		snap.Users,
		snap.Ramp.Round(time.Millisecond),
	)

	progress := renderProgress(snap.Elapsed, snap.Duration, snap.Phase)
	phaseLine := fmt.Sprintf("phase: %s · active users: %d/%d", snap.Phase, snap.ActiveUsers, snap.Users)

	kpi := renderKPI(snap)
	timelineChart := renderTimelineChart(snap.Timeline)
	opsTable := renderOperationsTable(snap.Operations)
	personas := renderPersonas(snap.Personas)
	footer := footerStyle.Render("q stop · reports → " + cfg.ReportDir)

	sections := []string{
		header,
		labelStyle.Render(meta),
		progress,
		labelStyle.Render(phaseLine),
		"",
		kpi,
		"",
		labelStyle.Render("Requests over time"),
		timelineChart,
		"",
		labelStyle.Render("Operations"),
		opsTable,
		"",
		labelStyle.Render("Personas"),
		personas,
		"",
		footer,
	}
	return strings.Join(sections, "\n")
}

func renderProgress(elapsed, total time.Duration, phase app.RunPhase) string {
	if total <= 0 {
		return valueStyle.Render(fmt.Sprintf("%s / —", elapsed.Round(time.Second)))
	}
	draining := phase == app.PhaseDraining
	pct := float64(elapsed) / float64(total)
	if pct > 1 && !draining {
		pct = 1
	}
	width := 30
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	bar := okStyle.Render(strings.Repeat("█", filled)) + labelStyle.Render(strings.Repeat("░", width-filled))
	if draining && elapsed > total {
		return fmt.Sprintf(
			"%s  %s / %s  draining",
			bar,
			elapsed.Round(time.Second),
			total.Round(time.Second),
		)
	}
	return fmt.Sprintf(
		"%s  %s / %s",
		bar,
		elapsed.Round(time.Second),
		total.Round(time.Second),
	)
}

func renderKPI(snap app.LiveSnapshot) string {
	errPct := 0.0
	if snap.TotalOperations > 0 {
		errPct = float64(snap.TotalErrors) / float64(snap.TotalOperations) * 100
	}
	errStyle := valueStyle
	if snap.TotalErrors > 0 {
		errStyle = warnStyle
	}
	lines := []string{
		strings.Join([]string{
			fmt.Sprintf("%s %s", labelStyle.Render("throughput"), valueStyle.Render(fmt.Sprintf("%.1f ops/s", snap.OperationsPerSecond))),
			fmt.Sprintf("%s %s", labelStyle.Render("errors"), errStyle.Render(fmt.Sprintf("%d (%.1f%%)", snap.TotalErrors, errPct))),
			fmt.Sprintf("%s %s", labelStyle.Render("p50"), valueStyle.Render(formatDuration(snap.P50))),
			fmt.Sprintf("%s %s", labelStyle.Render("p95"), valueStyle.Render(formatDuration(snap.P95))),
			fmt.Sprintf("%s %s", labelStyle.Render("p99"), valueStyle.Render(formatDuration(snap.P99))),
			fmt.Sprintf("%s %s", labelStyle.Render("total ops"), valueStyle.Render(fmt.Sprintf("%d", snap.TotalOperations))),
		}, "   "),
		fmt.Sprintf("%s %s   %s %s",
			labelStyle.Render("↑ sent"), valueStyle.Render(fmt.Sprintf("%s (%.1f MB/s)", app.FormatBytes(snap.BytesSent), snap.BytesSentPerSecond/(1<<20))),
			labelStyle.Render("↓ recv"), valueStyle.Render(fmt.Sprintf("%s (%.1f MB/s)", app.FormatBytes(snap.BytesReceived), snap.BytesReceivedPerSecond/(1<<20))),
		),
	}
	return strings.Join(lines, "\n")
}

func renderOperationsTable(ops []app.OperationLive) string {
	if len(ops) == 0 {
		return labelStyle.Render("  waiting for operations...")
	}

	const (
		colName = 22
		colCnt  = 7
		colErr  = 7
		colP95  = 9
		colRecv = 9
		colKind = 7
	)

	header := fmt.Sprintf(
		"  %-*s %*s %*s %*s %*s %-*s",
		colName, "Operation",
		colCnt, "Count",
		colErr, "Err%",
		colP95, "P95",
		colRecv, "Recv",
		colKind, "Kind",
	)
	lines := []string{labelStyle.Render(header)}

	for _, op := range ops {
		errPct := op.ErrorRate * 100
		errText := fmt.Sprintf("%5.1f%%", errPct)
		if op.Errors > 0 {
			errText = warnStyle.Render(errText)
		} else {
			errText = valueStyle.Render(errText)
		}
		marker := ""
		if errPct >= 5 {
			marker = warnStyle.Render(" ⚠")
		}
		avgRecv := int64(0)
		if op.Count > 0 {
			avgRecv = op.BytesReceived / op.Count
		}
		line := fmt.Sprintf(
			"  %-*s %*d %s %*s %*s %-*s%s",
			colName, op.Name,
			colCnt, op.Count,
			errText,
			colP95, formatDuration(op.P95),
			colRecv, app.FormatBytes(avgRecv),
			colKind, op.Kind,
			marker,
		)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func renderPersonas(personas map[string]int64) string {
	if len(personas) == 0 {
		return labelStyle.Render("  —")
	}
	order := []string{"shopper", "support", "analyst", "operations", "hr"}
	parts := make([]string, 0, len(order))
	for _, name := range order {
		if count, ok := personas[name]; ok {
			parts = append(parts, fmt.Sprintf("%s %d", name, count))
		}
	}
	for name, count := range personas {
		found := false
		for _, known := range order {
			if known == name {
				found = true
				break
			}
		}
		if !found {
			parts = append(parts, fmt.Sprintf("%s %d", name, count))
		}
	}
	return valueStyle.Render("  " + strings.Join(parts, " · "))
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	if d >= time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(100 * time.Microsecond).String()
}
