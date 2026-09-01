package application

import "myproject/order/internal/application/apperrors"

var (
	ErrInvalidInput = apperrors.ErrInvalidInput
	ErrNotFound     = apperrors.ErrNotFound
	ErrConflict     = apperrors.ErrConflict
)
