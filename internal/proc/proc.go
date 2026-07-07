package proc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type CellProcess struct {
	X    int
	Y    int
	Addr string
	cmd  *exec.Cmd
}

type Launcher struct {
	cellBin      string
	workDir      string
	debug        bool
	preparedOnce sync.Once
	preparedBin  string
	prepareErr   error
	tempDir      string
}

func NewLauncher(cellBin, workDir string, debug bool) *Launcher {
	return &Launcher{
		cellBin: cellBin,
		workDir: workDir,
		debug:   debug,
	}
}

func (l *Launcher) Cleanup() error {
	if l.tempDir == "" {
		return nil
	}
	err := os.RemoveAll(l.tempDir)
	l.tempDir = ""
	l.preparedBin = ""
	return err
}

func (l *Launcher) StartCell(ctx context.Context, x, y int, alive bool, generation, port int) (*CellProcess, error) {
	cmd, err := l.buildCommand(ctx, x, y, alive, generation, port)
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start cell (%d,%d): %w", x, y, err)
	}

	addr := "127.0.0.1:" + strconv.Itoa(port)
	if err := waitForTCP(ctx, addr, 5*time.Second); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("cell (%d,%d) did not listen on %s: %w", x, y, addr, err)
	}

	return &CellProcess{
		X:    x,
		Y:    y,
		Addr: addr,
		cmd:  cmd,
	}, nil
}

func (l *Launcher) buildCommand(ctx context.Context, x, y int, alive bool, generation, port int) (*exec.Cmd, error) {
	args := []string{
		"--x", strconv.Itoa(x),
		"--y", strconv.Itoa(y),
		"--port", strconv.Itoa(port),
		"--generation", strconv.Itoa(generation),
		"--alive=" + strconv.FormatBool(alive),
	}
	if l.debug {
		args = append(args, "--debug")
	}

	cellBin, err := l.resolveCellBinary(ctx)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, cellBin, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	return cmd, nil
}

func (l *Launcher) resolveCellBinary(ctx context.Context) (string, error) {
	if l.cellBin != "" {
		return l.cellBin, nil
	}

	exe, err := os.Executable()
	if err == nil {
		sibling := filepath.Join(filepath.Dir(exe), "proc-cell")
		if st, statErr := os.Stat(sibling); statErr == nil && !st.IsDir() {
			return sibling, nil
		}
	}

	l.preparedOnce.Do(func() {
		l.preparedBin, l.tempDir, l.prepareErr = l.buildCellBinary(ctx)
	})
	if l.prepareErr != nil {
		return "", l.prepareErr
	}
	return l.preparedBin, nil
}

func (l *Launcher) buildCellBinary(ctx context.Context) (string, string, error) {
	tempDir, err := os.MkdirTemp("", "proc-lifegame-cell-*")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir for proc-cell build: %w", err)
	}

	output := filepath.Join(tempDir, "proc-cell")
	cmd := exec.CommandContext(ctx, "go", "build", "-o", output, "./cmd/proc-cell")
	cmd.Dir = l.workDir
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("build proc-cell helper binary: %w", err)
	}
	return output, tempDir, nil
}

func waitForTCP(ctx context.Context, addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		dialer := &net.Dialer{Timeout: 200 * time.Millisecond}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (p *CellProcess) Wait() error {
	if p.cmd == nil {
		return nil
	}
	return p.cmd.Wait()
}

func (p *CellProcess) Stop(timeout time.Duration) error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		if killErr := p.cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return killErr
		}
		return <-done
	}
}
