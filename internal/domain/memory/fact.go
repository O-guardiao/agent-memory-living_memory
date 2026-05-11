package memory

type Fact struct {
	Memory
	Subject string `json:"subject,omitempty"`
	Object  string `json:"object,omitempty"`
}
