package application

import (
	"context"

	"myproject/iam/internal/application/ports"
	"myproject/iam/internal/domain"

	"github.com/google/uuid"
)

type RoleService struct {
	roles ports.RoleRepository
}

func NewRoleService(roles ports.RoleRepository) *RoleService { return &RoleService{roles: roles} }

func (s *RoleService) List(ctx context.Context) ([]domain.Role, error) {
	return s.roles.ListRoles(ctx)
}

func (s *RoleService) Create(ctx context.Context, role domain.Role) (domain.Role, error) {
	if role.Name == "" {
		return domain.Role{}, ErrInvalidInput
	}
	role.ID = uuid.New()
	return role, s.roles.CreateRole(ctx, role)
}

func (s *RoleService) Update(ctx context.Context, id string, role domain.Role) error {
	if role.Name == "" {
		return ErrInvalidInput
	}
	ok, err := s.roles.UpdateRole(ctx, id, role)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

func (s *RoleService) Delete(ctx context.Context, id string) error {
	ok, err := s.roles.DeleteRole(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}
