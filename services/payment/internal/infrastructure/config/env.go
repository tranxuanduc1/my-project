// Package config is the environment-backed configuration adapter.
package config

import "os"

func Env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
