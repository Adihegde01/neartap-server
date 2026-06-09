package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	Port                string
	FirebaseProjectID   string
	FirebaseCredentials string // path to service account JSON
	AllowedOrigins      []string
	Env                 string
}

// Load reads config from environment (with optional .env file)
func Load() *Config {
	// Load .env if it exists (non-fatal if missing in prod)
	if err := godotenv.Load(); err != nil {
		log.Println("[config] No .env file found, using environment variables")
	}

	port := getEnv("PORT", "8080")
	projectID := getEnv("FIREBASE_PROJECT_ID", "neartap-demo")
	credPath := getEnv("FIREBASE_CREDENTIALS_PATH", "./serviceAccountKey.json")
	env := getEnv("ENV", "development")

	origins := []string{
		"http://localhost:5173",
		"http://localhost:3000",
		"http://127.0.0.1:5173",
	}
	if frontendURL := os.Getenv("FRONTEND_URL"); frontendURL != "" {
		origins = append(origins, frontendURL)
	}

	return &Config{
		Port:                port,
		FirebaseProjectID:   projectID,
		FirebaseCredentials: credPath,
		AllowedOrigins:      origins,
		Env:                 env,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
