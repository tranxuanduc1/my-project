package bootstrap

import (
	"context"

	"myproject/iam/internal/application"
	"myproject/iam/internal/infrastructure/config"
	"myproject/iam/internal/infrastructure/postgres"
	"myproject/iam/internal/transport/httpapi"

	"github.com/gin-gonic/gin"
)

func env(k, d string) string { return config.Env(k, d) }

func Migrate() error {
	dsn := env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/iam?sslmode=disable")
	return postgres.Migrate(env("MIGRATIONS_PATH", "file://migrations"), dsn)
}

func Run() error {
	if err := Migrate(); err != nil {
		return err
	}
	store, err := postgres.Open(env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/iam?sslmode=disable"))
	if err != nil {
		return err
	}
	secret := []byte(env("JWT_SECRET", "dev-secret"))
	auth := application.NewAuthService(store, store, secret)
	if err := auth.SeedAdmin(context.Background(), env("IAM_ADMIN_EMAIL", "admin@example.com"), env("IAM_ADMIN_PASSWORD", "admin123456")); err != nil {
		return err
	}
	r := gin.Default()
	httpapi.NewHandler(store, auth, application.NewUserService(store), application.NewRoleService(store), secret).RegisterRoutes(r)
	return r.Run(":" + env("PORT", "8081"))
}
