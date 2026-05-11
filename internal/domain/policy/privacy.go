package policy

type PrivacyClass string

const (
	PrivacyPublic       PrivacyClass = "public"
	PrivacyInternal     PrivacyClass = "internal"
	PrivacyConfidential PrivacyClass = "confidential"
	PrivacyRestricted   PrivacyClass = "restricted"
)
