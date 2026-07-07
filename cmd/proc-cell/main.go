package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/tsolsikke/proc-lifegame/internal/life"
	"github.com/tsolsikke/proc-lifegame/internal/protocol"
)

type cell struct {
	state protocol.CellState
}

func main() {
	var (
		x          = flag.Int("x", 0, "cell x coordinate")
		y          = flag.Int("y", 0, "cell y coordinate")
		port       = flag.Int("port", 0, "TCP listen port")
		alive      = flag.Bool("alive", false, "initial alive state")
		generation = flag.Int("generation", 0, "initial generation")
		debug      = flag.Bool("debug", false, "enable debug cell logs")
	)
	flag.Parse()

	if *port <= 0 {
		log.Fatal("port must be greater than 0")
	}

	logger := log.New(os.Stderr, fmt.Sprintf("[cell %d,%d] ", *x, *y), log.LstdFlags|log.Lmicroseconds)
	c := &cell{
		state: protocol.CellState{
			X:          *x,
			Y:          *y,
			Alive:      *alive,
			Generation: *generation,
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		logger.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	if *debug {
		logger.Printf("listening on %s", listener.Addr())
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				if *debug {
					logger.Printf("shutting down")
				}
				return
			}
			logger.Printf("accept error: %v", err)
			continue
		}
		if err := c.handleConn(conn); err != nil {
			logger.Printf("request error: %v", err)
		}
	}
}

func (c *cell) handleConn(conn net.Conn) error {
	defer conn.Close()

	var req protocol.StepRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("decode request: %w", err)
	}

	resp := protocol.StepResponse{Type: "step_response"}
	if req.Type != "step" {
		resp.Error = "unsupported request type"
		return json.NewEncoder(conn).Encode(resp)
	}
	if req.Generation != c.state.Generation {
		resp.Error = fmt.Sprintf("generation mismatch: have=%d want=%d", c.state.Generation, req.Generation)
		resp.State = c.state
		return json.NewEncoder(conn).Encode(resp)
	}

	liveNeighbors := 0
	for _, neighbor := range req.Neighbors {
		if neighbor.Alive {
			liveNeighbors++
		}
	}

	c.state.Alive = life.NextAlive(c.state.Alive, liveNeighbors)
	c.state.Generation++
	resp.State = c.state
	return json.NewEncoder(conn).Encode(resp)
}
