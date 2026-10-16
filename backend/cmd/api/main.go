// Package main runs the RSP HTTP API server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/api"
	"github.com/magedmg/RSP-website/backend/internal/authadmin"
	"github.com/magedmg/RSP-website/backend/internal/authn"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/ratelimit"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	appEnv := env("APP_ENV", "development")
	if appEnv == "production" {
		if err := validateProductionConfig(os.Getenv); err != nil {
			logger.Error("invalid production configuration", "error", err)
			os.Exit(1)
		}
	}

	rateLimits, err := rateLimitsFromEnv(os.Getenv)
	if err != nil {
		logger.Error("invalid rate limit configuration", "error", err)
		os.Exit(1)
	}
	if rateLimits != (ratelimit.Limits{}) {
		logger.Info("rate limits overridden", "read", rateLimits.Read, "write", rateLimits.Write, "sensitive", rateLimits.Sensitive)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	db, err := dal.Open(context.Background(), databaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	for _, key := range []string{"AUTH_ISSUER", "AUTH_AUDIENCE", "AUTH_JWKS_URL", "IDENTITY_SERVICE_TOKEN"} {
		if os.Getenv(key) == "" {
			logger.Error(key + " is required")
			os.Exit(1)
		}
	}
	authenticator := api.BearerAuthenticator{
		Validator: &authn.Validator{
			Issuer:   os.Getenv("AUTH_ISSUER"),
			Audience: os.Getenv("AUTH_AUDIENCE"),
			JWKSURL:  os.Getenv("AUTH_JWKS_URL"),
		},
		Subjects: db,
	}
	authClient := authadmin.Client{
		BaseURL: env("AUTH_INTERNAL_URL", "http://auth:3001"),
		Token:   os.Getenv("IDENTITY_SERVICE_TOKEN"),
		HTTP:    &http.Client{Timeout: 5 * time.Second},
	}
	ready := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return db.Ping(ctx)
	}

	app := api.New(api.Config{
		DB:                 db,
		Authenticator:      authenticator,
		PublicOrigin:       env("PUBLIC_ORIGIN", "http://localhost:8080"),
		CursorSecret:       []byte(env("CURSOR_SECRET", "development-cursor-secret-change-me")),
		Logger:             logger,
		Ready:              ready,
		QueueLeetCodeSync:  db.QueueLeetCodeSync,
		SetAccountState:    authClient.SetAccountState,
		SendInvitation:     authClient.SendInvitation,
		RequestEmailChange: authClient.RequestEmailChange,
		GetMFAConfigured:   authClient.GetMFAConfigured,
		RateLimits:         rateLimits,
	})
	apiHandler := app.Handler()

	server := &http.Server{
		Addr:              env("API_ADDR", ":4000"),
		Handler:           apiHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("api listening", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func validateProductionConfig(getenv func(string) string) error {
	origin, err := url.Parse(getenv("PUBLIC_ORIGIN"))
	if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return errors.New("PUBLIC_ORIGIN must be an HTTPS origin without credentials, path, query, or fragment")
	}

	cursorSecret := getenv("CURSOR_SECRET")
	if len(cursorSecret) < 32 || strings.Contains(strings.ToLower(cursorSecret), "development") || strings.Contains(strings.ToLower(cursorSecret), "change-me") {
		return errors.New("CURSOR_SECRET must be at least 32 characters and must not be a development placeholder")
	}

	internalURL, err := url.Parse(getenv("AUTH_INTERNAL_URL"))
	if err != nil || internalURL.Host == "" || internalURL.Scheme != "http" && internalURL.Scheme != "https" {
		return errors.New("AUTH_INTERNAL_URL must be an absolute HTTP or HTTPS URL")
	}

	identityToken := getenv("IDENTITY_SERVICE_TOKEN")
	if len(identityToken) < 32 || strings.Contains(strings.ToLower(identityToken), "change-me") {
		return errors.New("IDENTITY_SERVICE_TOKEN must be at least 32 characters and must not be a placeholder")
	}

	return nil
}

// rateLimitsFromEnv reads optional per-minute overrides. Unset variables keep
// the defaults; a set but invalid value is an error rather than a silent default.
func rateLimitsFromEnv(getenv func(string) string) (ratelimit.Limits, error) {
	var limits ratelimit.Limits
	for _, setting := range []struct {
		key    string
		target *int
	}{
		{"API_RATE_LIMIT_READ_PER_MINUTE", &limits.Read},
		{"API_RATE_LIMIT_WRITE_PER_MINUTE", &limits.Write},
		{"API_RATE_LIMIT_SENSITIVE_PER_MINUTE", &limits.Sensitive},
	} {
		raw := strings.TrimSpace(getenv(setting.key))
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return ratelimit.Limits{}, errors.New(setting.key + " must be a positive integer")
		}
		*setting.target = value
	}
	return limits, nil
}

func env(k, f string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return f
}
