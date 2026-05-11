package tenant

type Namespace struct {
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
}
