package cursor

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
)

var ErrInvalid = errors.New("invalid cursor")

type payload struct {
	Version int    `json:"v"`
	After   string `json:"after"`
	Binding string `json:"binding"`
}

func Encode(secret []byte, after, binding string) (string, error) {
	raw, err := json.Marshal(payload{1, after, binding})
	if err != nil {
		return "", err
	}

	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(append(raw, mac.Sum(nil)...)), nil
}

func Decode(secret []byte, encoded, binding string) (string, error) {
	signed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(signed) <= sha256.Size {
		return "", ErrInvalid
	}

	raw, sig := signed[:len(signed)-sha256.Size], signed[len(signed)-sha256.Size:]
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(raw)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return "", ErrInvalid
	}
	var v payload
	if json.Unmarshal(raw, &v) != nil || v.Version != 1 || v.Binding != binding {
		return "", ErrInvalid
	}
	return v.After, nil
}
