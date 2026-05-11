package policy

import "time"

type DeletionRequest struct {
	TenantID string
	UserID   string
	MemoryID string
	Reason   string
}

type DeletionReceipt struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	MemoryID  string    `json:"memory_id"`
	DeletedAt time.Time `json:"deleted_at"`
	Reason    string    `json:"reason,omitempty"`
}
