package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/tsolsikke/proc-lifegame/internal/grid"
	"github.com/tsolsikke/proc-lifegame/internal/proc"
	"github.com/tsolsikke/proc-lifegame/internal/protocol"
	"github.com/tsolsikke/proc-lifegame/internal/ui"
)

type result struct {
	state protocol.CellState
	err   error
}

type simulation struct {
	cells            []*proc.CellProcess
	current          *grid.Board
	generation       int
	totalGenerations int
	pattern          string
	interval         time.Duration
	debug            bool
}

func main() {
	var (
		width       = flag.Int("width", 5, "board width")
		height      = flag.Int("height", 5, "board height")
		generations = flag.Int("generations", 5, "number of generations")
		basePort    = flag.Int("base-port", 15000, "starting TCP port for cell processes")
		pattern     = flag.String("pattern", "random", "initial pattern: random, single, block, blinker, toad, beacon, glider, lwss, glider-gun")
		seed        = flag.Int64("seed", time.Now().UnixNano(), "random seed used when pattern=random")
		cellBin     = flag.String("cell-bin", "", "path to proc-cell binary; default resolves sibling binary or uses `go run ./cmd/proc-cell`")
		interval    = flag.Duration("interval", time.Second, "generation update interval")
		uiMode      = flag.String("ui", "tui", "display mode: tui or cui")
		debug       = flag.Bool("debug", false, "enable debug output")
	)
	flag.Parse()

	if *interval <= 0 {
		log.Fatal("interval must be greater than 0")
	}

	debugEnabled := *debug
	logger := log.New(os.Stderr, "[coordinator] ", log.LstdFlags|log.Lmicroseconds)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cellCtx, cancelCells := context.WithCancel(ctx)
	defer cancelCells()

	board, err := grid.New(*width, *height)
	if err != nil {
		logger.Fatal(err)
	}
	switch *pattern {
	case "random":
		board.Randomize(rand.New(rand.NewSource(*seed)))
	default:
		if err := board.ApplyPattern(*pattern); err != nil {
			logger.Fatal(err)
		}
	}

	resolvedBasePort, err := findAvailableBasePort(*basePort, board.Width()*board.Height())
	if err != nil {
		logger.Fatal(err)
	}
	if debugEnabled && resolvedBasePort != *basePort {
		logger.Printf("base port %d is busy; using %d instead", *basePort, resolvedBasePort)
	}

	launcher := proc.NewLauncher(*cellBin, ".", debugEnabled)
	defer func() {
		if err := launcher.Cleanup(); err != nil && debugEnabled {
			logger.Printf("cleanup temporary proc-cell binary: %v", err)
		}
	}()
	cells, err := startCells(cellCtx, launcher, board, resolvedBasePort)
	if err != nil {
		stopCells(logger, cells, debugEnabled)
		cancelCells()
		logger.Fatal(err)
	}
	defer func() {
		stopCells(logger, cells, debugEnabled)
		cancelCells()
	}()

	sim := &simulation{
		cells:            cells,
		current:          board.Clone(),
		generation:       0,
		totalGenerations: *generations,
		pattern:          *pattern,
		interval:         *interval,
		debug:            debugEnabled,
	}

	initial := sim.Snapshot().WithStatus(ui.StatusRunning)
	switch *uiMode {
	case "tui":
		if err := ui.RunTUI(ctx, initial, sim.Step); err != nil && !errors.Is(err, context.Canceled) {
			logger.Fatal(err)
		}
	case "cui":
		if err := ui.RunCUI(ctx, os.Stdout, initial, sim.Step); err != nil && !errors.Is(err, context.Canceled) {
			logger.Fatal(err)
		}
	default:
		logger.Fatalf("unsupported ui mode %q", *uiMode)
	}
}

func startCells(ctx context.Context, launcher *proc.Launcher, board *grid.Board, basePort int) ([]*proc.CellProcess, error) {
	cells := make([]*proc.CellProcess, 0, board.Width()*board.Height())
	for y := 0; y < board.Height(); y++ {
		for x := 0; x < board.Width(); x++ {
			port := basePort + len(cells)
			cellProc, err := launcher.StartCell(ctx, x, y, board.Alive(x, y), 0, port)
			if err != nil {
				return cells, err
			}
			cells = append(cells, cellProc)
		}
	}
	return cells, nil
}

func findAvailableBasePort(start, count int) (int, error) {
	if start <= 0 {
		return 0, fmt.Errorf("base-port must be greater than 0")
	}
	if count <= 0 {
		return 0, fmt.Errorf("process count must be greater than 0")
	}

	const maxAttempts = 256
	for offset := 0; offset < maxAttempts; offset++ {
		candidate := start + offset*count
		ok, err := canBindPortRange(candidate, count)
		if err != nil {
			return 0, err
		}
		if ok {
			return candidate, nil
		}
	}

	return 0, fmt.Errorf("could not find %d contiguous free ports starting at %d", count, start)
}

func canBindPortRange(basePort, count int) (bool, error) {
	listeners := make([]net.Listener, 0, count)
	for i := 0; i < count; i++ {
		port := basePort + i
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			for _, listener := range listeners {
				_ = listener.Close()
			}
			var opErr *net.OpError
			if errors.As(err, &opErr) {
				return false, nil
			}
			return false, err
		}
		listeners = append(listeners, ln)
	}

	for _, listener := range listeners {
		if err := listener.Close(); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (s *simulation) Snapshot() ui.Model {
	return ui.Model{
		Mode:             "process-per-cell",
		IPC:              "TCP localhost",
		Grid:             s.current.Clone(),
		Width:            s.current.Width(),
		Height:           s.current.Height(),
		ProcessCount:     len(s.cells),
		TotalProcesses:   s.current.Width() * s.current.Height(),
		Generation:       s.generation,
		TotalGenerations: s.totalGenerations,
		Pattern:          s.pattern,
		Interval:         s.interval,
		Debug:            s.debug,
		Status:           ui.StatusRunning,
	}
}

func (s *simulation) Step(ctx context.Context) (ui.Model, error) {
	if s.generation >= s.totalGenerations {
		return s.Snapshot().WithStatus(ui.StatusCompleted), nil
	}

	next, err := stepGeneration(ctx, s.current, s.cells, s.generation)
	if err != nil {
		return s.Snapshot().WithError(err), err
	}

	s.current = next
	s.generation++
	model := s.Snapshot()
	if s.generation >= s.totalGenerations {
		model.Status = ui.StatusCompleted
	}
	return model, nil
}

func stepGeneration(ctx context.Context, current *grid.Board, cells []*proc.CellProcess, generation int) (*grid.Board, error) {
	next, err := grid.New(current.Width(), current.Height())
	if err != nil {
		return nil, err
	}

	results := make(chan result, len(cells))
	var wg sync.WaitGroup
	for _, cellProc := range cells {
		wg.Add(1)
		go func(cp *proc.CellProcess) {
			defer wg.Done()
			req := protocol.StepRequest{
				Type:       "step",
				Generation: generation,
				Neighbors:  current.Neighbors(cp.X, cp.Y),
			}
			state, err := sendStep(ctx, cp.Addr, req)
			results <- result{state: state, err: err}
		}(cellProc)
	}

	wg.Wait()
	close(results)

	for res := range results {
		if res.err != nil {
			return nil, res.err
		}
		next.Set(res.state.X, res.state.Y, res.state.Alive)
	}
	return next, nil
}

func sendStep(ctx context.Context, addr string, req protocol.StepRequest) (protocol.CellState, error) {
	var zero protocol.CellState

	dialer := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return zero, fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return zero, fmt.Errorf("encode request to %s: %w", addr, err)
	}

	var resp protocol.StepResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return zero, fmt.Errorf("decode response from %s: %w", addr, err)
	}
	if resp.Error != "" {
		return zero, errors.New(resp.Error)
	}
	return resp.State, nil
}

func stopCells(logger *log.Logger, cells []*proc.CellProcess, debug bool) {
	for _, cellProc := range cells {
		if err := cellProc.Stop(2 * time.Second); err != nil {
			if !debug {
				continue
			}
			if isExpectedTermination(err) {
				continue
			}
			logger.Printf("cell (%d,%d) exited with error: %v", cellProc.X, cellProc.Y, err)
		}
	}
}

func isExpectedTermination(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && status.Signal() == syscall.SIGTERM
}
