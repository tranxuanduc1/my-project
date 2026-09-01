// Package httpauth contains HTTP authentication and authorization middleware.
package httpauth

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Email string   `json:"email"`
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

func Middleware(secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("bad signing method")
			}
			return secret, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
			return
		}
		c.Set("claims", claims)
		c.Next()
	}
}

func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, current := range c.MustGet("claims").(*Claims).Roles {
			if current == role {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(403, gin.H{"error": "forbidden"})
	}
}
