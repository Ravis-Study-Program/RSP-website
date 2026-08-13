package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authn"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/httpapi"
	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	var repository store.Repository
	var ready func() error
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		postgres, err := store.Open(context.Background(), databaseURL)
		if err != nil {
			logger.Error("database connection failed", "error", err)
			os.Exit(1)
		}
		defer postgres.Close()
		repository = postgres
		ready = func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return postgres.Ping(ctx)
		}
	} else if env("APP_ENV", "development") == "development" {
		memory := store.NewMemory()
		memory.Users["dev-user"] = model.User{ID: "dev-user", Slug: "developer", Name: "Local Developer", Timezone: "Australia/Adelaide", GlobalRoles: []string{"system_admin"}, Revision: 1}
		repository = memory
	} else {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	mfa := time.Now().UTC()
	devActor := authz.Actor{UserID: "dev-user", EmailVerified: true, AccountState: authz.AccountActive, GlobalRoles: map[authz.GlobalRole]bool{authz.SystemAdmin: true}, MFAAt: &mfa}
	var authenticator httpapi.Authenticator
	if env("APP_ENV", "development") == "development" {
		authenticator = httpapi.AuthenticatorFunc(func(*http.Request) (authz.Actor, error) { return devActor, nil })
	} else {
		for _, key := range []string{"AUTH_ISSUER", "AUTH_AUDIENCE", "AUTH_JWKS_URL"} {
			if os.Getenv(key) == "" {
				logger.Error(key + " is required")
				os.Exit(1)
			}
		}
		authenticator = httpapi.BearerAuthenticator{Validator: &authn.Validator{Issuer: os.Getenv("AUTH_ISSUER"), Audience: os.Getenv("AUTH_AUDIENCE"), JWKSURL: os.Getenv("AUTH_JWKS_URL")}, Store: repository}
	}
	api := httpapi.New(httpapi.Config{Store: repository, Authenticator: authenticator, PublicOrigin: env("PUBLIC_ORIGIN", "http://localhost:8080"), CursorSecret: []byte(env("CURSOR_SECRET", "development-cursor-secret-change-me")), Logger: logger, Ready: ready})
	server := &http.Server{Addr: env("API_ADDR", ":4000"), Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second}
	logger.Info("api listening", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}
func env(k, f string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return f
}
