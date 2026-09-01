package ports

import (
	"context"

	"myproject/iam/internal/domain"
)

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type UserRepository interface {
	UserExistsByEmail(ctx context.Context, email string) (bool, error)
	CreateUser(ctx context.Context, user domain.User) error
	FindUserByEmail(ctx context.Context, email string) (domain.User, error)
	FindUserByID(ctx context.Context, id string) (domain.User, error)
	ListUsers(ctx context.Context) ([]domain.User, error)
	SetUserStatus(ctx context.Context, id, status string) (bool, error)
	SetUserRoles(ctx context.Context, id string, roleNames []string) error
}

type RoleRepository interface {
	FindRoleByName(ctx context.Context, name string) (domain.Role, error)
	ListRoles(ctx context.Context) ([]domain.Role, error)
	CreateRole(ctx context.Context, role domain.Role) error
	UpdateRole(ctx context.Context, id string, role domain.Role) (bool, error)
	DeleteRole(ctx context.Context, id string) (bool, error)
}
