package memory

type Procedure struct {
	Memory
	Trigger string   `json:"trigger,omitempty"`
	Steps   []string `json:"steps,omitempty"`
}
