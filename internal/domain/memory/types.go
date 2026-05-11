package memory

type Type string

const (
	TypeEpisode    Type = "episode"
	TypeFact       Type = "fact"
	TypePreference Type = "preference"
	TypeProcedure  Type = "procedure"
	TypeRelation   Type = "relation"
	TypeNote       Type = "note"
)

type Status string

const (
	StatusActive     Status = "active"
	StatusSuperseded Status = "superseded"
	StatusDeleted    Status = "deleted"
	StatusExpired    Status = "expired"
)

type SafetyLabel string

const (
	SafetyPublic       SafetyLabel = "public"
	SafetyInternal     SafetyLabel = "internal"
	SafetySensitive    SafetyLabel = "sensitive"
	SafetyPII          SafetyLabel = "pii"
	SafetyCredential   SafetyLabel = "credential"
	SafetyPromptAttack SafetyLabel = "prompt_attack"
)
