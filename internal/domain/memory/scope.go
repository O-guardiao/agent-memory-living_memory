package memory

type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeTenant  Scope = "tenant"
	ScopeOrg     Scope = "organization"
	ScopeProject Scope = "project"
	ScopeAgent   Scope = "agent"
	ScopeUser    Scope = "user"
	ScopeSession Scope = "session"
	ScopeThread  Scope = "thread"
)
