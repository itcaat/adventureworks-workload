package tui

import (
	"context"
	"fmt"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"adventureworks-workload/internal/app"
)

type snapshotMsg app.LiveSnapshot

type finishedMsg struct {
	report app.Report
	err    error
}

type Model struct {
	cfg    app.Config
	cancel context.CancelFunc

	mu     sync.Mutex
	snap   app.LiveSnapshot
	done   bool
	report app.Report
	runErr error
}

func newModel(cfg app.Config, cancel context.CancelFunc) *Model {
	return &Model{
		cfg:    cfg,
		cancel: cancel,
		snap: app.LiveSnapshot{
			RunID:     cfg.RunID(),
			Profile:   cfg.Profile,
			WriteMode: cfg.WriteMode,
			Users:     cfg.Users,
			Duration:  cfg.Duration,
			Ramp:      cfg.Ramp,
			Phase:     app.PhaseRamping,
		},
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case snapshotMsg:
		m.mu.Lock()
		m.snap = app.LiveSnapshot(msg)
		m.mu.Unlock()
	case finishedMsg:
		m.mu.Lock()
		m.done = true
		m.report = msg.report
		m.runErr = msg.err
		m.mu.Unlock()
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) View() string {
	m.mu.Lock()
	snap := m.snap
	done := m.done
	report := m.report
	m.mu.Unlock()

	if done {
		return renderFinished(report)
	}
	return renderDashboard(m.cfg, snap)
}

func renderFinished(report app.Report) string {
	if report.RunID == "" {
		return "\nRun stopped.\n"
	}
	p95 := formatDuration(report.P95)
	return fmt.Sprintf(
		"\nRun %s finished: %d operations, %d errors, %.2f ops/sec, p95 %s\n",
		report.RunID,
		report.TotalOperations,
		report.TotalErrors,
		report.OperationsPerSecond,
		p95,
	)
}
