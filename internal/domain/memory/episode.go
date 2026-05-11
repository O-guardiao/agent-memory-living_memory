package memory

type Episode struct {
	Memory
	Actors []string `json:"actors,omitempty"`
}
