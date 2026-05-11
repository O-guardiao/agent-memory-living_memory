package memory

import "errors"

var (
	ErrNotFound       = errors.New("memory not found")
	ErrDeleted        = errors.New("memory deleted")
	ErrInvalidScope   = errors.New("invalid memory scope")
	ErrInvalidContent = errors.New("invalid memory content")
)
