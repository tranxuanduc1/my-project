package application

import (
	"context"
	"strings"
	"time"

	"myproject/iam/internal/application/apperrors"
	"myproject/iam/internal/application/ports"
	"myproject/iam/internal/domain"
	"myproject/iam/internal/transport/httpauth"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User = domain.User
type Role = domain.Role
type claims = httpauth.Claims

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type AuthService struct {
	users  ports.UserRepository
	roles  ports.RoleRepository
	secret []byte
}

func NewAuthService(users ports.UserRepository, roles ports.RoleRepository, secret []byte) *AuthService {
	return &AuthService{users: users, roles: roles, secret: secret}
}

func (s *AuthService) SeedAdmin(ctx context.Context, email, password string) error {
	email = strings.ToLower(email)
	exists, err := s.users.UserExistsByEmail(ctx, email)
	if err != nil || exists {
		return err
	}
	role, err := s.roles.FindRoleByName(ctx, "admin")
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.users.CreateUser(ctx, domain.User{ID: uuid.New(), Email: email, PasswordHash: string(hash), Status: "active", Roles: []domain.Role{role}})
}

func (s *AuthService) Register(ctx context.Context, email, password string) (domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") || len(password) < 4 {
		return domain.User{}, apperrors.ErrInvalidInput
	}
	role, err := s.roles.FindRoleByName(ctx, "customer")
	if err != nil {
		return domain.User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, err
	}
	user := domain.User{ID: uuid.New(), Email: email, PasswordHash: string(hash), Status: "active", Roles: []domain.Role{role}}
	if err := s.users.CreateUser(ctx, user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (TokenResponse, error) {
	user, err := s.users.FindUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil || user.Status != "active" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return TokenResponse{}, apperrors.ErrUnauthorized
	}
	roles := make([]string, 0, len(user.Roles))
	for _, role := range user.Roles {
		roles = append(roles, role.Name)
	}
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Email: user.Email,
		Roles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	})
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{AccessToken: signed, TokenType: "Bearer", ExpiresIn: 86400}, nil
}

func (s *AuthService) Me(ctx context.Context, id string) (domain.User, error) {
	return s.users.FindUserByID(ctx, id)
}
