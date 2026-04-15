package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	TranscriberBackend    string
	WhisperURL            string
	WhisperHealthInterval int

	CloudflareAccountID string
	CloudflareAPIToken  string
	CloudflareLanguage  string

	AdminPhone string
	Port       string
}

func Load() *Config {
	// Tries to load .env from current directory (useful for dev)
	godotenv.Load()
	// Tries to load from system-wide path (useful for RPM/prod)
	godotenv.Load("/etc/tekstobot.env")

	cfg := &Config{
		DBHost:                getEnv("DB_HOST", "localhost"),
		DBPort:                getEnv("DB_PORT", "5432"),
		DBUser:                getEnv("DB_USER", "postgres"),
		DBPassword:            getEnv("DB_PASSWORD", "postgres"),
		DBName:                getEnv("DB_NAME", "tekstobot"),
		TranscriberBackend:    getEnv("TRANSCRIBER_BACKEND", "local"),
		WhisperURL:            getEnv("WHISPER_URL", "http://localhost:8000"),
		WhisperHealthInterval: getEnvAsInt("WHISPER_HEALTH_INTERVAL", 30),
		CloudflareAccountID:   getEnv("CLOUDFLARE_ACCOUNT_ID", ""),
		CloudflareAPIToken:    getEnv("CLOUDFLARE_API_TOKEN", ""),
		CloudflareLanguage:    getEnv("CLOUDFLARE_WHISPER_LANGUAGE", ""),
		AdminPhone:            getEnv("ADMIN_PHONE", ""),
		Port:                  getEnv("PORT", "8080"),
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if valueStr, exists := os.LookupEnv(key); exists {
		var value int
		if _, err := fmt.Sscanf(valueStr, "%d", &value); err == nil {
			return value
		}
	}
	return fallback
}
