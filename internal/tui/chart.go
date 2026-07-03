package tui

import (
	"fmt"
	"strings"

	"adventureworks-workload/internal/app"
)

func renderTimelineChart(buckets []app.TimelineBucket) string {
	if len(buckets) == 0 {
		return labelStyle.Render("  waiting for requests...")
	}

	chartBuckets := buckets
	maxTotal := int64(0)
	for _, bucket := range chartBuckets {
		if bucket.Operations > maxTotal {
			maxTotal = bucket.Operations
		}
	}
	if maxTotal == 0 {
		maxTotal = 1
	}

	const height = 8
	lines := make([]string, height)
	for row := 0; row < height; row++ {
		var parts []string
		for _, bucket := range chartBuckets {
			parts = append(parts, timelineCell(bucket, row, height, maxTotal))
		}
		y := int64(height-1-row) * maxTotal / int64(height)
		if row == 0 {
			y = maxTotal
		}
		label := labelStyle.Render(fmt.Sprintf("%4d │", y))
		lines[row] = label + strings.Join(parts, "")
	}

	var b strings.Builder
	for _, line := range lines {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString("       ")
	b.WriteString(labelStyle.Render("└"))
	b.WriteString(labelStyle.Render(strings.Repeat("─", len(chartBuckets))))
	b.WriteByte('\n')
	b.WriteString("        ")
	b.WriteString(labelStyle.Render(formatChartTime(chartBuckets[0].Start)))
	if len(chartBuckets) > 1 {
		last := chartBuckets[len(chartBuckets)-1]
		padding := len(chartBuckets) - len(formatChartTime(chartBuckets[0].Start)) - len(formatChartTime(last.End))
		if padding < 1 {
			padding = 1
		}
		b.WriteString(strings.Repeat(" ", padding))
		b.WriteString(labelStyle.Render(formatChartTime(last.End)))
	}
	b.WriteByte('\n')
	b.WriteString("       ")
	b.WriteString(okStyle.Render("█"))
	b.WriteString(labelStyle.Render(" ok  "))
	b.WriteString(warnStyle.Render("█"))
	b.WriteString(labelStyle.Render(" errors"))
	return strings.TrimRight(b.String(), "\n")
}

func timelineCell(bucket app.TimelineBucket, row, height int, maxTotal int64) string {
	if maxTotal == 0 || bucket.Operations == 0 {
		return labelStyle.Render("░")
	}

	totalH := int(bucket.Operations * int64(height) / maxTotal)
	if totalH == 0 && bucket.Operations > 0 {
		totalH = 1
	}
	errH := int(bucket.Errors * int64(height) / maxTotal)
	okH := totalH - errH

	rowFromBottom := height - 1 - row
	if rowFromBottom >= totalH {
		return labelStyle.Render("░")
	}
	if rowFromBottom < okH {
		return okStyle.Render("█")
	}
	return warnStyle.Render("█")
}

func formatChartTime(d interface{ Seconds() float64 }) string {
	secs := int(d.Seconds())
	if secs >= 60 {
		return fmt.Sprintf("%dm%02ds", secs/60, secs%60)
	}
	return fmt.Sprintf("%ds", secs)
}
