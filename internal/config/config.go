package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	APIKey   string
	Theme    string
	Model    string
	Language string
	Output   string
}

func LoadConfig() *Config {
	// Load .env if it exists
	_ = godotenv.Load()

	return &Config{
		APIKey:   os.Getenv("OPENAI_API_KEY"),
		Theme:    getEnv("GLIMPSE_THEME", "seriph"),
		Model:    getEnv("GLIMPSE_MODEL", "gpt-4o"),
		Language: getEnv("GLIMPSE_LANGUAGE", "en"),
		Output:   getEnv("GLIMPSE_OUTPUT", "slides.md"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
