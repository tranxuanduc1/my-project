package application

import (
	"context"

	"myproject/iam/internal/application/ports"
	"myproject/iam/internal/domain"
)

type UserService struct {
	users ports.UserRepository
}

func NewUserService(users ports.UserRepository) *UserService { return &UserService{users: users} }

func (s *UserService) List(ctx context.Context) ([]domain.User, error) {
	return s.users.ListUsers(ctx)
}

func (s *UserService) Get(ctx context.Context, id string) (domain.User, error) {
	return s.users.FindUserByID(ctx, id)
}

func (s *UserService) SetStatus(ctx context.Context, id, status string) error {
	if status != "active" && status != "disabled" {
		return ErrInvalidInput
	}
	ok, err := s.users.SetUserStatus(ctx, id, status)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

func (s *UserService) SetRoles(ctx context.Context, id string, roleNames []string) error {
	if len(roleNames) == 0 {
		return ErrInvalidInput
	}
	return s.users.SetUserRoles(ctx, id, roleNames)
}
