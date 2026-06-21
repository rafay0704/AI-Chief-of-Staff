package domain

import "errors"

// Sentinel errors used across layers. The HTTP layer maps these to status codes
// in one place (see internal/http/handler/errors.go).
var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource already exists")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrValidation   = errors.New("validation failed")
)
