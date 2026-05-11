package policy

import "time"

type RetentionPolicy struct {
	TenantID          string
	DefaultTTL        time.Duration
	DeleteExpiredHard bool
}
