package postgres

import (
	"context"
	"errors"

	"myproject/iam/internal/application/apperrors"
	"myproject/iam/internal/domain"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func Open(dsn string) (*Store, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func Migrate(path, dsn string) error {
	m, err := migrate.New(path, dsn+"&x-migrations-table=iam_schema_migrations")
	if err != nil {
		return err
	}
	defer m.Close()
	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

func (s *Store) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (s *Store) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&domain.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

func (s *Store) CreateUser(ctx context.Context, user domain.User) error {
	err := s.db.WithContext(ctx).Create(&user).Error
	if err != nil {
		return apperrors.ErrConflict
	}
	return nil
}

func (s *Store) FindUserByEmail(ctx context.Context, email string) (domain.User, error) {
	var user domain.User
	err := s.db.WithContext(ctx).Preload("Roles").Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, apperrors.ErrNotFound
	}
	return user, err
}

func (s *Store) FindUserByID(ctx context.Context, id string) (domain.User, error) {
	var user domain.User
	err := s.db.WithContext(ctx).Preload("Roles").First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, apperrors.ErrNotFound
	}
	return user, err
}

func (s *Store) ListUsers(ctx context.Context) ([]domain.User, error) {
	var users []domain.User
	err := s.db.WithContext(ctx).Preload("Roles").Order("created_at desc").Find(&users).Error
	return users, err
}

func (s *Store) SetUserStatus(ctx context.Context, id, status string) (bool, error) {
	res := s.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).Update("status", status)
	return res.RowsAffected > 0, res.Error
}

func (s *Store) SetUserRoles(ctx context.Context, id string, roleNames []string) error {
	var user domain.User
	if err := s.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		return err
	}
	var roles []domain.Role
	if err := s.db.WithContext(ctx).Where("name IN ?", roleNames).Find(&roles).Error; err != nil {
		return err
	}
	if len(roles) != len(roleNames) {
		return apperrors.ErrUnknownRole
	}
	return s.db.WithContext(ctx).Model(&user).Association("Roles").Replace(roles)
}

func (s *Store) FindRoleByName(ctx context.Context, name string) (domain.Role, error) {
	var role domain.Role
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Role{}, apperrors.ErrNotFound
	}
	return role, err
}

func (s *Store) ListRoles(ctx context.Context) ([]domain.Role, error) {
	var roles []domain.Role
	err := s.db.WithContext(ctx).Order("name").Find(&roles).Error
	return roles, err
}

func (s *Store) CreateRole(ctx context.Context, role domain.Role) error {
	if err := s.db.WithContext(ctx).Create(&role).Error; err != nil {
		return apperrors.ErrConflict
	}
	return nil
}

func (s *Store) UpdateRole(ctx context.Context, id string, role domain.Role) (bool, error) {
	res := s.db.WithContext(ctx).Model(&domain.Role{}).Where("id = ?", id).Updates(map[string]any{"name": role.Name, "description": role.Description})
	return res.RowsAffected > 0, res.Error
}

func (s *Store) DeleteRole(ctx context.Context, id string) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Table("user_roles").Where("role_id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, apperrors.ErrRoleInUse
	}
	res := s.db.WithContext(ctx).Delete(&domain.Role{}, "id = ?", id)
	return res.RowsAffected > 0, res.Error
}
