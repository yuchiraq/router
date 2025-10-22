package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config structure for application configuration

type Config struct {
	AdminUser string
	AdminPass string
}

// Load loads configuration from .env file

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	return &Config{
		AdminUser: getEnv("ADMIN_USER", "admin"),
		AdminPass: getEnv("ADMIN_PASS", "password"),
	}
}

// Helper to get an environment variable or return a default value

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
