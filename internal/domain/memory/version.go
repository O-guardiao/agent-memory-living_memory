package memory

import "time"

type Version struct {
	ID           string    `json:"id"`
	MemoryID     string    `json:"memory_id"`
	Version      int       `json:"version"`
	Content      string    `json:"content"`
	ChangeReason string    `json:"change_reason,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
