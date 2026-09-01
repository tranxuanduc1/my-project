package apperrors

import "errors"

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
	ErrUnknownRole  = errors.New("unknown role")
	ErrRoleInUse    = errors.New("role is in use")
)
