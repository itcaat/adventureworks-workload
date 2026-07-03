package tui

import (
	"context"
	"sync"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"adventureworks-workload/internal/app"
)

type snapshotMsg app.LiveSnapshot

type finishedMsg struct {
	report   app.Report
	err      error
	mdPath   string
	jsonPath string
}

type Model struct {
	cfg    app.Config
	cancel context.CancelFunc

	mu         sync.Mutex
	snap       app.LiveSnapshot
	showReport bool
	report     app.Report
	runErr     error
	mdPath     string
	jsonPath   string

	width    int
	height   int
	viewport viewport.Model
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
		if m.showReport {
			return m, nil
		}
		m.mu.Lock()
		m.snap = app.LiveSnapshot(msg)
		m.mu.Unlock()
	case finishedMsg:
		m.mu.Lock()
		m.report = msg.report
		m.runErr = msg.err
		m.mdPath = msg.mdPath
		m.jsonPath = msg.jsonPath
		m.showReport = true
		m.mu.Unlock()
		m.initReportViewport()
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.showReport {
			m.initReportViewport()
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	case tea.KeyMsg:
		if m.showReport {
			switch msg.String() {
			case "q", "ctrl+c", "esc":
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "q", "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
	}

	if m.showReport {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) View() string {
	m.mu.Lock()
	showReport := m.showReport
	snap := m.snap
	report := m.report
	mdPath := m.mdPath
	jsonPath := m.jsonPath
	m.mu.Unlock()

	if showReport {
		header := titleStyle.Render(" awload report — " + report.RunID + " ")
		footer := footerStyle.Render("↑/↓ pgup/pgdn scroll · q quit")
		if m.viewport.Height == 0 {
			return header + "\n" + renderReportContent(m.cfg, report, mdPath, jsonPath) + "\n" + footer
		}
		return header + "\n" + m.viewport.View() + "\n" + footer
	}
	return renderDashboard(m.cfg, snap)
}

func (m *Model) initReportViewport() {
	if m.width <= 0 {
		m.width = 80
	}
	if m.height <= 4 {
		m.height = 24
	}
	h := max(1, m.height-4)
	w := max(1, m.width)

	m.mu.Lock()
	report := m.report
	mdPath := m.mdPath
	jsonPath := m.jsonPath
	m.mu.Unlock()

	content := renderReportContent(m.cfg, report, mdPath, jsonPath)
	if m.viewport.Height == 0 {
		m.viewport = viewport.New(w, h)
		m.viewport.SetContent(content)
		return
	}
	m.viewport.Width = w
	m.viewport.Height = h
	m.viewport.SetContent(content)
}
