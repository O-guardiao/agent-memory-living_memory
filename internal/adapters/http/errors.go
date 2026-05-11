package httpadapter

import "errors"

var (
	errMethodNotAllowed   = errors.New("method not allowed")
	errNotFound           = errors.New("not found")
	errServiceUnavailable = errors.New("service unavailable")
)
