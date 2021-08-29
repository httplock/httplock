package storage

import (
	"errors"
)

var (
	// ErrNotFound is returned when requested object is not found
	ErrNotFound = errors.New("Not found")
)
