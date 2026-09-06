package authn

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidatesIssuerAudienceSignatureKidExpiryAndVerification(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	encode := func(v *big.Int) string { return base64.RawURLEncoding.EncodeToString(v.Bytes()) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA",
			"kid": "k1",
			"n":   encode(key.N),
			"e":   encode(big.NewInt(int64(key.E))),
		}}})
	}))
	defer server.Close()
	now := time.Now()
	makeToken := func(verified bool, aud string, expires time.Time) string {
		c := Claims{
			EmailVerified:    verified,
			AccountState:     "active",
			SecurityVersion:  1,
			MFAVerified:      true,
			MFAVerifiedAt:    &ClaimDateTime{Time: now},
			RegisteredClaims: jwt.RegisteredClaims{Issuer: "auth", Subject: "auth-user", Audience: jwt.ClaimStrings{aud}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expires)},
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
		tok.Header["kid"] = "k1"
		raw, _ := tok.SignedString(key)
		return raw
	}
	v := Validator{
		Issuer:   "auth",
		Audience: "rsp-api",
		JWKSURL:  server.URL,
	}
	claims, err := v.Validate(context.Background(), makeToken(true, "rsp-api", now.Add(time.Minute)))
	if err != nil || claims.Subject != "auth-user" {
		t.Fatalf("valid token: %v", err)
	}

	for name, raw := range map[string]string{"unverified": makeToken(false, "rsp-api", now.Add(time.Minute)), "audience": makeToken(true, "other", now.Add(time.Minute)), "expired": makeToken(true, "rsp-api", now.Add(-time.Minute))} {
		if _, err := v.Validate(context.Background(), raw); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func TestAcceptsRFC3339AndNumericMFAVerifiedAtClaims(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for name, raw := range map[string][]byte{
		"RFC3339": []byte(`"` + now.Format(time.RFC3339Nano) + `"`),
		"numeric": []byte(fmt.Sprintf("%d", now.Unix())),
	} {
		t.Run(name, func(t *testing.T) {
			var claim ClaimDateTime
			if err := json.Unmarshal(raw, &claim); err != nil {
				t.Fatal(err)
			}
			if !claim.Time.Equal(now) {
				t.Fatalf("claim time=%s want=%s", claim.Time, now)
			}
		})
	}
}

func TestRefreshesKnownKeyAfterTTLAndRejectsRetiredKey(t *testing.T) {
	retired, _ := rsa.GenerateKey(rand.Reader, 2048)
	replacement, _ := rsa.GenerateKey(rand.Reader, 2048)
	encode := func(v *big.Int) string { return base64.RawURLEncoding.EncodeToString(v.Bytes()) }
	var rotated atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		key, kid := retired, "retired"
		if rotated.Load() {
			key, kid = replacement, "replacement"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA",
			"kid": kid,
			"n":   encode(key.N),
			"e":   encode(big.NewInt(int64(key.E))),
		}}})
	}))
	defer server.Close()

	now := time.Now()
	claims := Claims{
		EmailVerified:    true,
		AccountState:     "active",
		SecurityVersion:  1,
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "auth", Subject: "auth-user", Audience: jwt.ClaimStrings{"rsp-api"}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute))},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "retired"
	raw, _ := token.SignedString(retired)
	validator := Validator{
		Issuer:   "auth",
		Audience: "rsp-api",
		JWKSURL:  server.URL,
		TTL:      time.Millisecond,
	}
	if _, err := validator.Validate(context.Background(), raw); err != nil {
		t.Fatalf("initial token rejected: %v", err)
	}

	rotated.Store(true)
	time.Sleep(2 * time.Millisecond)
	if _, err := validator.Validate(context.Background(), raw); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("retired cached key accepted after TTL: %v", err)
	}
}

func TestRejectsUnavailableStateAndMissingSecurityVersion(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	encode := func(v *big.Int) string { return base64.RawURLEncoding.EncodeToString(v.Bytes()) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA",
			"kid": "k1",
			"n":   encode(key.N),
			"e":   encode(big.NewInt(int64(key.E))),
		}}})
	}))
	defer server.Close()
	now := time.Now()
	sign := func(state string, version int64) string {
		claims := Claims{
			EmailVerified:    true,
			AccountState:     state,
			SecurityVersion:  version,
			RegisteredClaims: jwt.RegisteredClaims{Issuer: "auth", Subject: "auth-user", Audience: jwt.ClaimStrings{"rsp-api"}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute))},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = "k1"
		raw, _ := token.SignedString(key)
		return raw
	}
	validator := Validator{
		Issuer:   "auth",
		Audience: "rsp-api",
		JWKSURL:  server.URL,
	}
	for name, raw := range map[string]string{
		"suspended":        sign("suspended", 2),
		"deletion pending": sign("deletion_pending", 2),
		"missing version":  sign("active", 0),
	} {
		if _, err := validator.Validate(context.Background(), raw); !errors.Is(err, ErrAccountUnavailable) {
			t.Fatalf("%s: expected account unavailable, got %v", name, err)
		}
	}
}

func TestValidateClaimsIsPure(t *testing.T) {
	claims := Claims{
		EmailVerified:   true,
		AccountState:    "active",
		SecurityVersion: 1,
	}
	if err := ValidateClaims(claims); err != nil {
		t.Fatal(err)
	}

	claims.AccountState = "suspended"
	if err := ValidateClaims(claims); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatalf("suspended claims returned %v", err)
	}
}
