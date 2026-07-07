package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds all runtime configuration, loaded from environment variables.
type Config struct {
	DatabaseURL   string
	JWTSecret     string
	JWTTTL        time.Duration
	HTTPAddr      string
	YouTubeAPIKey string // for the YouTube Data API v3 search
	AppBaseURL    string // public base URL, used to build password-reset links
	AdminEmails   []string
	UploadsDir    string // where shopping receipts are stored

	// SMTP (optional). When SMTPHost is empty, emails are logged instead of sent.
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

// Load reads configuration from the environment, applying sensible defaults.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:   env("DATABASE_URL", "postgres://helpparty:helpparty@localhost:5432/helpparty?sslmode=disable"),
		JWTSecret:     env("JWT_SECRET", "dev-insecure-secret-change-me"),
		HTTPAddr:      env("HTTP_ADDR", ":8080"),
		YouTubeAPIKey: env("YOUTUBE_API_KEY", ""),
		AppBaseURL:    env("APP_BASE_URL", "http://localhost:8080"),
		AdminEmails:   splitList(env("ADMIN_EMAILS", defaultAdmins)),
		UploadsDir:    env("UPLOADS_DIR", "./data/uploads"),
		JWTTTL:        7 * 24 * time.Hour,
		SMTPHost:      env("SMTP_HOST", ""),
		SMTPPort:      env("SMTP_PORT", "587"),
		SMTPUser:      env("SMTP_USER", ""),
		SMTPPass:      env("SMTP_PASS", ""),
		SMTPFrom:      env("SMTP_FROM", "help-party <no-reply@help-party.local>"),
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

// defaultAdmins is intentionally empty: admin emails are provided at runtime via
// the ADMIN_EMAILS env var (kept out of source control / the public repo).
const defaultAdmins = ""

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(strings.ToLower(p)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
