package authn

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidatesIssuerAudienceSignatureKidExpiryAndVerification(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	encode := func(v *big.Int) string { return base64.RawURLEncoding.EncodeToString(v.Bytes()) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "k1", "n": encode(key.N), "e": encode(big.NewInt(int64(key.E)))}}})
	}))
	defer server.Close()
	now := time.Now()
	makeToken := func(verified bool, aud string, expires time.Time) string {
		c := Claims{EmailVerified: verified, AccountState: "active", MFAVerified: true, MFAVerifiedAt: jwt.NewNumericDate(now), RegisteredClaims: jwt.RegisteredClaims{Issuer: "auth", Subject: "auth-user", Audience: jwt.ClaimStrings{aud}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expires)}}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
		tok.Header["kid"] = "k1"
		raw, _ := tok.SignedString(key)
		return raw
	}
	v := Validator{Issuer: "auth", Audience: "rsp-api", JWKSURL: server.URL}
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
