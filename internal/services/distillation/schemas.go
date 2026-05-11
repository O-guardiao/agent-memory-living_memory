package distillation

type ExtractedMemorySchema struct {
	Type       string  `json:"type"`
	Content    string  `json:"content"`
	Scope      string  `json:"scope"`
	Confidence float64 `json:"confidence"`
	Importance float64 `json:"importance"`
}
