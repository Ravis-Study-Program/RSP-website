// Package main runs the RSP HTTP API server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authadmin"
	"github.com/magedmg/RSP-website/backend/internal/authn"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/httpapi"
	"github.com/magedmg/RSP-website/backend/internal/postgres"
	"github.com/magedmg/RSP-website/backend/internal/store"
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

	var repository httpapi.Repository
	var ready func() error
	var syncLeetCode func(context.Context, string, string) error
	var setAccountState func(context.Context, string, string, string, string) error
	var getMFAState func(context.Context, string) (bool, error)

	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		postgres, err := postgres.Open(context.Background(), databaseURL)
		if err != nil {
			logger.Error("database connection failed", "error", err)
			os.Exit(1)
		}

		defer postgres.Close()
		repository = postgres
		syncLeetCode = postgres.QueueLeetCodeSync
		ready = func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return postgres.Ping(ctx)
		}
	} else if appEnv == "development" {
		memory := store.NewMemory()
		memory.Users["dev-admin"] = accounts.User{ID: "dev-admin", Slug: "dev-admin", Name: "Dev Admin", Email: "admin@rsp.local", AccountState: "active", Timezone: "Australia/Adelaide", TimezoneConfigured: true, GlobalRoles: []string{"system_admin"}, Revision: 1}
		repository = memory
	} else {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	mfa := time.Now().UTC()
	devActor := authz.Actor{UserID: "dev-admin", EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{authz.SystemAdmin: true}, MFAAt: &mfa}
	var authenticator httpapi.Authenticator

	if os.Getenv("DEV_AUTH_BYPASS") == "true" {
		if appEnv != "development" {
			logger.Error("DEV_AUTH_BYPASS is forbidden outside development")
			os.Exit(1)
		}
		authenticator = httpapi.AuthenticatorFunc(func(*http.Request) (authz.Actor, error) { return devActor, nil })
	} else {
		for _, key := range []string{"AUTH_ISSUER", "AUTH_AUDIENCE", "AUTH_JWKS_URL", "IDENTITY_SERVICE_TOKEN"} {
			if os.Getenv(key) == "" {
				logger.Error(key + " is required")
				os.Exit(1)
			}
		}
		authenticator = httpapi.BearerAuthenticator{Validator: &authn.Validator{Issuer: os.Getenv("AUTH_ISSUER"), Audience: os.Getenv("AUTH_AUDIENCE"), JWKSURL: os.Getenv("AUTH_JWKS_URL")}, Subjects: repository}
		authClient := authadmin.Client{BaseURL: env("AUTH_INTERNAL_URL", "http://auth:3001"), Token: os.Getenv("IDENTITY_SERVICE_TOKEN"), HTTP: &http.Client{Timeout: 5 * time.Second}}
		setAccountState = authClient.SetAccountState
		getMFAState = authClient.MFAConfigured
	}

	api := httpapi.New(httpapi.Config{Store: repository, Authenticator: authenticator, PublicOrigin: env("PUBLIC_ORIGIN", "http://localhost:8080"), CursorSecret: []byte(env("CURSOR_SECRET", "development-cursor-secret-change-me")), Logger: logger, Ready: ready, SyncLeetCode: syncLeetCode, SetAccountState: setAccountState, GetMFAState: getMFAState})
	apiHandler := api.Handler()

	server := &http.Server{Addr: env("API_ADDR", ":4000"), Handler: apiHandler, ReadHeaderTimeout: 5 * time.Second}
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

func env(k, f string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return f
}
