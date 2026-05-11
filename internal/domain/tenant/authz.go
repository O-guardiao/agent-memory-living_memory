package tenant

func SameTenant(required, actual string) bool {
	return required != "" && required == actual
}
