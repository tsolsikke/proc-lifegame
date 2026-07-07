package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time

type stepDoneMsg struct {
	model Model
	err   error
}

type tuiModel struct {
	ctx       context.Context
	state     Model
	step      StepFunc
	paused    bool
	stepping  bool
	quitting  bool
	header    lipgloss.Style
	value     lipgloss.Style
	gridStyle lipgloss.Style
	help      lipgloss.Style
	error     lipgloss.Style
}

func RunTUI(ctx context.Context, initial Model, step StepFunc) error {
	m := tuiModel{
		ctx:       ctx,
		state:     initial.WithStatus(StatusRunning),
		step:      step,
		header:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")),
		value:     lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		gridStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("120")),
		help:      lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
		error:     lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m tuiModel) Init() tea.Cmd {
	return tickCmd(m.state.Interval)
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			m.state = m.state.WithStatus(StatusStopped)
			return m, tea.Quit
		case " ":
			if m.state.IsComplete() {
				return m, nil
			}
			m.paused = !m.paused
			if m.paused {
				m.state = m.state.WithStatus(StatusPaused)
			} else {
				m.state = m.state.WithStatus(StatusRunning)
			}
		case "n":
			if m.paused && !m.stepping && !m.state.IsComplete() {
				m.stepping = true
				return m, m.stepCmd()
			}
		case "+", "=":
			m.state = m.state.WithInterval(fasterInterval(m.state.Interval))
		case "-":
			m.state = m.state.WithInterval(slowerInterval(m.state.Interval))
		}
	case tickMsg:
		if m.quitting || m.state.IsComplete() {
			return m, nil
		}
		if m.paused || m.stepping {
			return m, tickCmd(m.state.Interval)
		}
		m.stepping = true
		return m, tea.Batch(m.stepCmd(), tickCmd(m.state.Interval))
	case stepDoneMsg:
		m.stepping = false
		if msg.err != nil {
			m.state = m.state.WithError(msg.err)
			return m, tea.Quit
		}
		m.state = msg.model.WithInterval(m.state.Interval)
		if m.state.IsComplete() {
			m.state = m.state.WithStatus(StatusCompleted)
		} else if m.paused {
			m.state = m.state.WithStatus(StatusPaused)
		} else {
			m.state = m.state.WithStatus(StatusRunning)
		}
	}
	return m, nil
}

func (m tuiModel) View() string {
	if m.quitting {
		return ""
	}

	lines := []string{
		m.header.Render("proc-lifegame"),
		"",
		fmt.Sprintf("Mode        : %s", m.value.Render(m.state.Mode)),
		fmt.Sprintf("IPC         : %s", m.value.Render(m.state.IPC)),
		fmt.Sprintf("Grid        : %s", m.value.Render(fmt.Sprintf("%d x %d", m.state.Width, m.state.Height))),
		fmt.Sprintf("Processes   : %s", m.value.Render(fmt.Sprintf("%d / %d", m.state.ProcessCount, m.state.TotalProcesses))),
		fmt.Sprintf("Generation  : %s", m.value.Render(fmt.Sprintf("%d / %d", m.state.Generation, m.state.TotalGenerations))),
		fmt.Sprintf("Pattern     : %s", m.value.Render(m.state.Pattern)),
		fmt.Sprintf("Interval    : %s", m.value.Render(m.state.Interval.String())),
		fmt.Sprintf("Debug       : %s", m.value.Render(fmt.Sprintf("%t", m.state.Debug))),
		fmt.Sprintf("Status      : %s", m.value.Render(string(m.state.Status))),
		"",
		m.gridStyle.Render(m.state.RenderBoard(true)),
		"",
		m.help.Render("[space] pause/resume  [n] step  [+/-] speed  [q] quit"),
	}
	if m.state.ErrorMessage != "" {
		lines = append(lines, "", m.error.Render("Error: "+m.state.ErrorMessage))
	}
	return strings.Join(lines, "\n")
}

func (m tuiModel) stepCmd() tea.Cmd {
	return func() tea.Msg {
		next, err := m.step(m.ctx)
		return stepDoneMsg{model: next, err: err}
	}
}

func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fasterInterval(interval time.Duration) time.Duration {
	if interval <= 50*time.Millisecond {
		return 50 * time.Millisecond
	}
	next := interval / 2
	if next < 50*time.Millisecond {
		return 50 * time.Millisecond
	}
	return next
}

func slowerInterval(interval time.Duration) time.Duration {
	if interval >= 10*time.Second {
		return 10 * time.Second
	}
	next := interval * 2
	if next > 10*time.Second {
		return 10 * time.Second
	}
	return next
}
