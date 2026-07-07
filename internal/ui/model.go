package ui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tsolsikke/proc-lifegame/internal/grid"
)

type Status string

const (
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusStopped   Status = "stopped"
	StatusError     Status = "error"
)

type Model struct {
	Mode             string
	IPC              string
	Grid             *grid.Board
	Width            int
	Height           int
	ProcessCount     int
	TotalProcesses   int
	Generation       int
	TotalGenerations int
	Pattern          string
	Interval         time.Duration
	Debug            bool
	Status           Status
	ErrorMessage     string
}

type StepFunc func(context.Context) (Model, error)

func (m Model) WithStatus(status Status) Model {
	m.Status = status
	return m
}

func (m Model) WithInterval(interval time.Duration) Model {
	m.Interval = interval
	return m
}

func (m Model) WithError(err error) Model {
	if err == nil {
		m.ErrorMessage = ""
		return m
	}
	m.Status = StatusError
	m.ErrorMessage = err.Error()
	return m
}

func (m Model) IsComplete() bool {
	return m.Generation >= m.TotalGenerations
}

func (m Model) HeaderLines() []string {
	lines := []string{
		"proc-lifegame",
		"",
		fmt.Sprintf("Mode        : %s", m.Mode),
		fmt.Sprintf("IPC         : %s", m.IPC),
		fmt.Sprintf("Grid        : %d x %d", m.Width, m.Height),
		fmt.Sprintf("Processes   : %d / %d", m.ProcessCount, m.TotalProcesses),
		fmt.Sprintf("Generation  : %d / %d", m.Generation, m.TotalGenerations),
		fmt.Sprintf("Pattern     : %s", m.Pattern),
		fmt.Sprintf("Interval    : %s", m.Interval),
		fmt.Sprintf("Debug       : %t", m.Debug),
		fmt.Sprintf("Status      : %s", m.Status),
	}
	if m.ErrorMessage != "" {
		lines = append(lines, fmt.Sprintf("Error       : %s", m.ErrorMessage))
	}
	return lines
}

func (m Model) RenderBoard(spaced bool) string {
	if m.Grid == nil {
		return ""
	}

	var sb strings.Builder
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			cell := "."
			if m.Grid.Alive(x, y) {
				if spaced {
					cell = "█"
				} else {
					cell = "#"
				}
			}
			sb.WriteString(cell)
			if spaced && x < m.Width-1 {
				sb.WriteByte(' ')
			}
		}
		if y < m.Height-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func WriteLines(w io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}
