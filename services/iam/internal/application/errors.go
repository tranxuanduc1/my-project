package application

import "myproject/iam/internal/application/apperrors"

var (
	ErrInvalidInput = apperrors.ErrInvalidInput
	ErrNotFound     = apperrors.ErrNotFound
	ErrConflict     = apperrors.ErrConflict
	ErrUnauthorized = apperrors.ErrUnauthorized
	ErrUnknownRole  = apperrors.ErrUnknownRole
	ErrRoleInUse    = apperrors.ErrRoleInUse
)
