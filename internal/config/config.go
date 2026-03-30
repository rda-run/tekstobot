package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	WhisperURL string
	Port       string
}

func Load() *Config {
	// Tries to load .env from current directory (useful for dev)
	godotenv.Load()
	// Tries to load from system-wide path (useful for RPM/prod)
	godotenv.Load("/etc/tekstobot.env")

	cfg := &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "tekstobot"),
		WhisperURL: getEnv("WHISPER_URL", "http://localhost:8000"),
		Port:       getEnv("PORT", "8080"),
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
