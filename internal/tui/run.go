package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"adventureworks-workload/internal/app"
)

type WorkloadFunc func(ctx context.Context, onProgress app.ProgressFunc) (app.Report, error)

func RunWithWorkload(parent context.Context, cfg app.Config, workload WorkloadFunc) (app.Report, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	model := newModel(cfg, cancel)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx))

	var (
		report app.Report
		runErr error
	)
	done := make(chan struct{})

	go func() {
		defer close(done)
		report, runErr = workload(ctx, func(snap app.LiveSnapshot) {
			program.Send(snapshotMsg(snap))
		})

		mdPath, jsonPath, writeErr := app.SaveReports(report, cfg)
		if writeErr != nil && runErr == nil {
			runErr = writeErr
		}
		program.Send(finishedMsg{
			report:   report,
			err:      runErr,
			mdPath:   mdPath,
			jsonPath: jsonPath,
		})
	}()

	if _, err := program.Run(); err != nil {
		cancel()
		<-done
		return app.Report{}, err
	}
	<-done
	return report, runErr
}
