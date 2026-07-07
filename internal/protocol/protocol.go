package protocol

type CellState struct {
	X          int  `json:"x"`
	Y          int  `json:"y"`
	Alive      bool `json:"alive"`
	Generation int  `json:"generation"`
}

type NeighborState struct {
	X     int  `json:"x"`
	Y     int  `json:"y"`
	Alive bool `json:"alive"`
}

type StepRequest struct {
	Type       string          `json:"type"`
	Generation int             `json:"generation"`
	Neighbors  []NeighborState `json:"neighbors"`
}

type StepResponse struct {
	Type  string    `json:"type"`
	State CellState `json:"state"`
	Error string    `json:"error,omitempty"`
}
