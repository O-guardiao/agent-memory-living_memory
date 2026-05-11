package memory

type Preference struct {
	Memory
	Category string  `json:"category,omitempty"`
	Strength float64 `json:"strength,omitempty"`
}
