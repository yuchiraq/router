package config

import "os"

// Config holds the application configuration
type Config struct {
	Username string
	Password string
}

// New creates a new Config from environment variables
func New() *Config {
	return &Config{
		Username: os.Getenv("ROUTER_USERNAME"),
		Password: os.Getenv("ROUTER_PASSWORD"),
	}
}
