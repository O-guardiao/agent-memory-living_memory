package memory

import "time"

type RelationType string

const (
	RelationSupports    RelationType = "supports"
	RelationContradicts RelationType = "contradicts"
	RelationSupersedes  RelationType = "supersedes"
	RelationDerivedFrom RelationType = "derived_from"
	RelationRelatedTo   RelationType = "related_to"
)

type Relation struct {
	ID           string       `json:"id"`
	TenantID     string       `json:"tenant_id"`
	FromMemoryID string       `json:"from_memory_id"`
	ToMemoryID   string       `json:"to_memory_id"`
	Type         RelationType `json:"type"`
	Confidence   float64      `json:"confidence"`
	CreatedAt    time.Time    `json:"created_at"`
}
