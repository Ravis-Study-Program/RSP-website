// Package authn validates backend access tokens.
package authn

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrInvalidToken is a public value used by the backend.
	ErrInvalidToken = errors.New("invalid access token")
	// ErrUnverified is a public value used by the backend.
	ErrUnverified = errors.New("email is not verified")
	// ErrAccountUnavailable is a public value used by the backend.
	ErrAccountUnavailable = errors.New("account is unavailable")
)

// Claims represents a backend data structure.
type Claims struct {
	EmailVerified   bool           `json:"emailVerified"`
	AccountState    string         `json:"accountState,omitempty"`
	SecurityVersion int64          `json:"securityVersion,omitempty"`
	MFAVerified     bool           `json:"mfaVerified,omitempty"`
	MFAVerifiedAt   *ClaimDateTime `json:"mfaVerifiedAt,omitempty"`
	jwt.RegisteredClaims
}

// ClaimDateTime accepts both Better Auth's RFC3339 access-policy value and a
// standard JWT NumericDate. This keeps key/session rotation compatible without
// requiring the browser-facing auth contract to expose numeric timestamps.
type ClaimDateTime struct{ time.Time }

// UnmarshalJSON performs the operation.
func (v *ClaimDateTime) UnmarshalJSON(raw []byte) error {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		parsed, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return fmt.Errorf("invalid RFC3339 claim time: %w", err)
		}

		v.Time = parsed.UTC()
		return nil
	}

	var numeric jwt.NumericDate
	if err := json.Unmarshal(raw, &numeric); err != nil {
		return fmt.Errorf("invalid claim time: %w", err)
	}

	v.Time = numeric.Time.UTC()
	return nil
}

// MarshalJSON performs the operation.
func (v ClaimDateTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Time.UTC().Format(time.RFC3339Nano))
}

type jwk struct{ Kty, Kid, Alg, Use, N, E, Crv, X, Y string }

type jwks struct {
	Keys []jwk `json:"keys"`
}

// Validator represents a backend data structure.
type Validator struct {
	Issuer, Audience, JWKSURL string
	Client                    *http.Client
	TTL                       time.Duration
	mu                        sync.RWMutex
	keys                      map[string]any
	loadedAt                  time.Time
}

// ValidateClaims checks the application-level claims after JWT signature and
// registered-claim validation have completed. It is pure and easy to test
// without a network or key cache.
func ValidateClaims(claims Claims) error {
	if !claims.EmailVerified {
		return ErrUnverified
	}
	if claims.AccountState != "active" || claims.SecurityVersion < 1 {
		return ErrAccountUnavailable
	}
	return nil
}

// Validate validates a value.
func (v *Validator) Validate(ctx context.Context, raw string) (Claims, error) {
	if v.Client == nil {
		v.Client = &http.Client{Timeout: 5 * time.Second}
	}
	if v.TTL == 0 {
		v.TTL = 5 * time.Minute
	}
	if err := v.refresh(ctx, false); err != nil {
		return Claims{}, fmt.Errorf("%w: key refresh failed", ErrInvalidToken)
	}

	claims := Claims{}
	parser := jwt.NewParser(jwt.WithIssuer(v.Issuer), jwt.WithAudience(v.Audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithLeeway(30*time.Second), jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "EdDSA", "ES256"}))
	token, err := parser.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, ErrInvalidToken
		}
		key, ok := v.key(kid)
		if !ok {
			if err := v.refresh(ctx, true); err != nil {
				return nil, err
			}

			key, ok = v.key(kid)
		}
		if !ok {
			return nil, fmt.Errorf("%w: unknown key id", ErrInvalidToken)
		}
		return key, nil
	})
	if err != nil || !token.Valid || claims.Subject == "" {
		return Claims{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if err := ValidateClaims(claims); err != nil {
		return Claims{}, err
	}
	return claims, nil
}

func (v *Validator) key(kid string) (any, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok := v.keys[kid]
	return key, ok
}

func (v *Validator) refresh(ctx context.Context, force bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !force && len(v.keys) > 0 && time.Since(v.loadedAt) < v.TTL {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.JWKSURL, nil)
	if err != nil {
		return err
	}

	res, err := v.Client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks response %d", res.StatusCode)
	}
	var set jwks
	if err := json.NewDecoder(http.MaxBytesReader(nil, res.Body, 1<<20)).Decode(&set); err != nil {
		return err
	}

	keys := map[string]any{}
	for _, item := range set.Keys {
		key, err := parseJWK(item)
		if err != nil {
			return fmt.Errorf("key %q: %w", item.Kid, err)
		}
		if item.Kid == "" {
			return errors.New("jwks key missing kid")
		}
		keys[item.Kid] = key
	}
	if len(keys) == 0 {
		return errors.New("empty jwks")
	}
	v.keys = keys
	v.loadedAt = time.Now()
	return nil
}

func parseJWK(j jwk) (any, error) {
	switch j.Kty {
	case "RSA":
		n, err := decodeBig(j.N)
		if err != nil {
			return nil, err
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(j.E)
		if err != nil {
			return nil, err
		}
		e := 0
		for _, b := range eBytes {
			e = e<<8 + int(b)
		}
		if e <= 0 {
			return nil, errors.New("invalid exponent")
		}
		return &rsa.PublicKey{N: n, E: e}, nil
	case "OKP":
		if j.Crv != "Ed25519" {
			return nil, errors.New("unsupported OKP curve")
		}
		x, err := base64.RawURLEncoding.DecodeString(j.X)
		if err != nil || len(x) != ed25519.PublicKeySize {
			return nil, errors.New("invalid Ed25519 key")
		}
		return ed25519.PublicKey(x), nil
	case "EC":
		if j.Crv != "P-256" {
			return nil, errors.New("unsupported EC curve")
		}
		x, err := base64.RawURLEncoding.DecodeString(j.X)
		if err != nil || len(x) != 32 {
			return nil, errors.New("invalid P-256 x coordinate")
		}
		y, err := base64.RawURLEncoding.DecodeString(j.Y)
		if err != nil || len(y) != 32 {
			return nil, errors.New("invalid P-256 y coordinate")
		}
		encoded := make([]byte, 1+len(x)+len(y))
		encoded[0] = 4
		copy(encoded[1:], x)
		copy(encoded[1+len(x):], y)
		key, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), encoded)
		if err != nil {
			return nil, errors.New("invalid P-256 public key")
		}
		return key, nil
	default:
		return nil, errors.New("unsupported key type")
	}
}

func decodeBig(v string) (*big.Int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil || len(raw) == 0 {
		return nil, errors.New("invalid key integer")
	}
	return new(big.Int).SetBytes(raw), nil
}
