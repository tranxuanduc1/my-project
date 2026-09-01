package application

import "myproject/payment/internal/application/apperrors"

var (
	ErrNotFound       = apperrors.ErrNotFound
	ErrConflict       = apperrors.ErrConflict
	ErrReconcileOrder = apperrors.ErrReconcileOrder
)
