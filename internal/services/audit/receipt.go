package auditservice

// Add immutable audit receipts here for compliance-sensitive deployments.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/agent-memory/agent-memory/internal/ports"
)

// Receipt is a hash-chained record of a destructive action (delete,
// redact, expire). Each receipt commits to the previous one, so any
// tampering breaks chain verification.
type Receipt struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Action    string    `json:"action"`
	SubjectID string    `json:"subject_id"`
	Reason    string    `json:"reason,omitempty"`
	PrevHash  string    `json:"prev_hash,omitempty"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
}

func NewReceipt(prevHash, tenantID, action, subjectID, reason string, idgen ports.IDGenerator, now time.Time) Receipt {
	receipt := Receipt{
		ID:        idgen.NewID("rcpt"),
		TenantID:  tenantID,
		Action:    action,
		SubjectID: subjectID,
		Reason:    reason,
		PrevHash:  prevHash,
		CreatedAt: now,
	}
	receipt.Hash = receiptHash(receipt)
	return receipt
}

func receiptHash(r Receipt) string {
	sum := sha256.Sum256([]byte(r.PrevHash + "|" + r.TenantID + "|" + r.Action + "|" + r.SubjectID + "|" + r.Reason + "|" + r.CreatedAt.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(sum[:])
}

// ReceiptLog keeps the chain in memory and, when an object store is
// configured, persists each receipt under receipts/<tenant>/<id>.json.
type ReceiptLog struct {
	mu       sync.Mutex
	last     string
	receipts []Receipt
	store    ports.ObjectStore
}

func NewReceiptLog(store ports.ObjectStore) *ReceiptLog {
	return &ReceiptLog{store: store}
}

// Append issues a chained receipt for the action and returns it.
func (l *ReceiptLog) Append(ctx context.Context, tenantID, action, subjectID, reason string, idgen ports.IDGenerator, now time.Time) (Receipt, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	receipt := NewReceipt(l.last, tenantID, action, subjectID, reason, idgen, now)
	if l.store != nil {
		payload, err := json.Marshal(receipt)
		if err != nil {
			return Receipt{}, err
		}
		key := fmt.Sprintf("receipts/%s/%s.json", tenantID, receipt.ID)
		if err := l.store.Put(ctx, key, payload); err != nil {
			return Receipt{}, err
		}
	}
	l.receipts = append(l.receipts, receipt)
	l.last = receipt.Hash
	return receipt, nil
}

// Verify recomputes the chain and fails on any break or tampering.
func (l *ReceiptLog) Verify() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	prev := ""
	for i, receipt := range l.receipts {
		if receipt.PrevHash != prev {
			return fmt.Errorf("receipt %d: chain break", i)
		}
		if receiptHash(receipt) != receipt.Hash {
			return fmt.Errorf("receipt %d: hash mismatch", i)
		}
		prev = receipt.Hash
	}
	return nil
}

// Receipts returns a copy of the in-memory chain.
func (l *ReceiptLog) Receipts() []Receipt {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Receipt, len(l.receipts))
	copy(out, l.receipts)
	return out
}
