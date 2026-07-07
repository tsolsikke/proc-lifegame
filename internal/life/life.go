package life

func NextAlive(alive bool, liveNeighbors int) bool {
	if alive {
		return liveNeighbors == 2 || liveNeighbors == 3
	}
	return liveNeighbors == 3
}
