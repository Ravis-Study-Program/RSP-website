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
	ErrInvalidToken = errors.New("invalid access token")
	ErrUnverified   = errors.New("email is not verified")
)

type Claims struct {
	EmailVerified bool             `json:"emailVerified"`
	MFAAt         *jwt.NumericDate `json:"mfaAt,omitempty"`
	jwt.RegisteredClaims
}
type jwk struct{ Kty, Kid, Alg, Use, N, E, Crv, X, Y string }
type jwks struct {
	Keys []jwk `json:"keys"`
}
type Validator struct {
	Issuer, Audience, JWKSURL string
	Client                    *http.Client
	TTL                       time.Duration
	mu                        sync.RWMutex
	keys                      map[string]any
	loadedAt                  time.Time
}

func (v *Validator) Validate(ctx context.Context, raw string) (Claims, error) {
	if v.Client == nil {
		v.Client = &http.Client{Timeout: 5 * time.Second}
	}
	if v.TTL == 0 {
		v.TTL = 5 * time.Minute
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
	if !claims.EmailVerified {
		return Claims{}, ErrUnverified
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
		x, err := decodeBig(j.X)
		if err != nil {
			return nil, err
		}
		y, err := decodeBig(j.Y)
		if err != nil {
			return nil, err
		}
		if !elliptic.P256().IsOnCurve(x, y) {
			return nil, errors.New("point is not on curve")
		}
		return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
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
