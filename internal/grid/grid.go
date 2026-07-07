package grid

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/tsolsikke/proc-lifegame/internal/protocol"
)

type Board struct {
	width  int
	height int
	cells  []bool
}

type patternDef struct {
	name   string
	width  int
	height int
	rows   []string
}

func New(width, height int) (*Board, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid board size %dx%d", width, height)
	}
	return &Board{
		width:  width,
		height: height,
		cells:  make([]bool, width*height),
	}, nil
}

func (b *Board) Width() int  { return b.width }
func (b *Board) Height() int { return b.height }

func (b *Board) Index(x, y int) int {
	return y*b.width + x
}

func (b *Board) InBounds(x, y int) bool {
	return x >= 0 && x < b.width && y >= 0 && y < b.height
}

func (b *Board) Set(x, y int, alive bool) {
	if !b.InBounds(x, y) {
		return
	}
	b.cells[b.Index(x, y)] = alive
}

func (b *Board) Alive(x, y int) bool {
	if !b.InBounds(x, y) {
		return false
	}
	return b.cells[b.Index(x, y)]
}

func (b *Board) StateAt(x, y, generation int) protocol.CellState {
	return protocol.CellState{
		X:          x,
		Y:          y,
		Alive:      b.Alive(x, y),
		Generation: generation,
	}
}

func (b *Board) Clone() *Board {
	dup := &Board{
		width:  b.width,
		height: b.height,
		cells:  make([]bool, len(b.cells)),
	}
	copy(dup.cells, b.cells)
	return dup
}

func (b *Board) Randomize(rng *rand.Rand) {
	for i := range b.cells {
		b.cells[i] = rng.Intn(2) == 1
	}
}

func SupportedPatterns() []string {
	names := make([]string, 0, len(patterns)+1)
	names = append(names, "random")
	for _, pattern := range patterns {
		names = append(names, pattern.name)
	}
	return names
}

func (b *Board) ApplyPattern(name string) error {
	name = strings.ToLower(name)
	if name == "random" {
		return nil
	}

	pattern, ok := findPattern(name)
	if !ok {
		return fmt.Errorf("unsupported pattern %q", name)
	}
	if b.width < pattern.width || b.height < pattern.height {
		return fmt.Errorf(
			"pattern %q requires board at least %dx%d, got %dx%d",
			name,
			pattern.width,
			pattern.height,
			b.width,
			b.height,
		)
	}

	b.stampCentered(pattern.rows)
	return nil
}

func (b *Board) stampCentered(pattern []string) {
	if len(pattern) == 0 {
		return
	}

	patternHeight := len(pattern)
	patternWidth := 0
	for _, row := range pattern {
		if len(row) > patternWidth {
			patternWidth = len(row)
		}
	}

	startX := (b.width - patternWidth) / 2
	startY := (b.height - patternHeight) / 2

	for y, row := range pattern {
		for x, cell := range row {
			if cell == '#' {
				b.Set(startX+x, startY+y, true)
			}
		}
	}
}

func findPattern(name string) (patternDef, bool) {
	for _, pattern := range patterns {
		if pattern.name == name {
			return pattern, true
		}
	}
	return patternDef{}, false
}

var patterns = []patternDef{
	{
		name:   "single",
		width:  1,
		height: 1,
		rows: []string{
			"#",
		},
	},
	{
		name:   "block",
		width:  2,
		height: 2,
		rows: []string{
			"##",
			"##",
		},
	},
	{
		name:   "blinker",
		width:  3,
		height: 1,
		rows: []string{
			"###",
		},
	},
	{
		name:   "toad",
		width:  4,
		height: 2,
		rows: []string{
			".###",
			"###.",
		},
	},
	{
		name:   "beacon",
		width:  4,
		height: 4,
		rows: []string{
			"##..",
			"##..",
			"..##",
			"..##",
		},
	},
	{
		name:   "glider",
		width:  3,
		height: 3,
		rows: []string{
			".#.",
			"..#",
			"###",
		},
	},
	{
		name:   "lwss",
		width:  5,
		height: 4,
		rows: []string{
			".##.#",
			"#....",
			"#...#",
			"#####",
		},
	},
	{
		name:   "glider-gun",
		width:  36,
		height: 9,
		rows: []string{
			"........................#...........",
			"......................#.#...........",
			"............##......##............##",
			"...........#...#....##............##",
			"##........#.....#...##..............",
			"##........#...#.##....#.#...........",
			"..........#.....#.......#...........",
			"...........#...#....................",
			"............##......................",
		},
	},
}

func (b *Board) Neighbors(x, y int) []protocol.NeighborState {
	neighbors := make([]protocol.NeighborState, 0, 8)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := x+dx, y+dy
			neighbors = append(neighbors, protocol.NeighborState{
				X:     nx,
				Y:     ny,
				Alive: b.Alive(nx, ny),
			})
		}
	}
	return neighbors
}

func (b *Board) String() string {
	var sb strings.Builder
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			if b.Alive(x, y) {
				sb.WriteByte('#')
			} else {
				sb.WriteByte('.')
			}
		}
		if y < b.height-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
