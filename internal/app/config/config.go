package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	HTTPAddr    string
}

func Load() (Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	return Config{
		DatabaseURL: dbURL,
		HTTPAddr:    addr,
	}, nil
}
