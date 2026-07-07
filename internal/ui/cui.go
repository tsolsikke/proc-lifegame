package ui

import (
	"context"
	"fmt"
	"io"
	"time"
)

func RunCUI(ctx context.Context, w io.Writer, initial Model, step StepFunc) error {
	model := initial.WithStatus(StatusRunning)
	if err := renderCUI(w, model); err != nil {
		return err
	}

	ticker := time.NewTicker(model.Interval)
	defer ticker.Stop()

	for !model.IsComplete() {
		select {
		case <-ctx.Done():
			model = model.WithStatus(StatusStopped).WithError(ctx.Err())
			return renderCUI(w, model)
		case <-ticker.C:
			next, err := step(ctx)
			if err != nil {
				model = model.WithError(err)
				_ = renderCUI(w, model)
				return err
			}
			model = next
			if model.IsComplete() {
				model = model.WithStatus(StatusCompleted)
			} else {
				model = model.WithStatus(StatusRunning)
			}
			if err := renderCUI(w, model); err != nil {
				return err
			}
			ticker.Reset(model.Interval)
		}
	}

	return nil
}

func renderCUI(w io.Writer, model Model) error {
	if _, err := fmt.Fprint(w, "\033[H\033[2J"); err != nil {
		return err
	}
	lines := model.HeaderLines()
	lines = append(lines, "", model.RenderBoard(false))
	return WriteLines(w, lines)
}
