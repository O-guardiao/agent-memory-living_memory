package policy

type Permission string

const (
	PermissionReadMemory   Permission = "memory:read"
	PermissionWriteMemory  Permission = "memory:write"
	PermissionDeleteMemory Permission = "memory:delete"
	PermissionReadTrace    Permission = "trace:read"
)
