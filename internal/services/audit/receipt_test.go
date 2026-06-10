package auditservice

import (
	"context"
	"testing"
	"time"

	"github.com/agent-memory/agent-memory/internal/adapters/system"
)

func TestReceiptChainAppendsAndVerifies(t *testing.T) {
	log := NewReceiptLog(nil)
	idgen := system.NewIDGenerator()
	ctx := context.Background()
	now := time.Unix(1000, 0)

	first, err := log.Append(ctx, "tenant_t", "delete", "mem_1", "user request", idgen, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.PrevHash != "" || first.Hash == "" {
		t.Fatalf("unexpected genesis receipt: %+v", first)
	}
	second, err := log.Append(ctx, "tenant_t", "redact", "mem_2", "", idgen, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if second.PrevHash != first.Hash {
		t.Fatal("second receipt must chain to first")
	}
	if err := log.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestReceiptChainDetectsTampering(t *testing.T) {
	log := NewReceiptLog(nil)
	idgen := system.NewIDGenerator()
	ctx := context.Background()
	_, _ = log.Append(ctx, "t", "delete", "m1", "", idgen, time.Unix(1, 0))
	_, _ = log.Append(ctx, "t", "delete", "m2", "", idgen, time.Unix(2, 0))

	log.receipts[0].Reason = "tampered"
	if err := log.Verify(); err == nil {
		t.Fatal("expected verification failure after tampering")
	}
}
