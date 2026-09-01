package apperrors

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrConflict       = errors.New("conflict")
	ErrReconcileOrder = errors.New("order reconciliation failed")
)
