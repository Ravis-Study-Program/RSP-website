// Package authadmin provides a client for the auth administration API.
package authadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

type SetAccountStateInput struct {
	AuthUserID  string
	State       string
	Reason      string
	ActorUserID string
}

func (c Client) SetAccountState(ctx context.Context, input SetAccountStateInput) error {
	body, err := json.Marshal(map[string]string{"state": input.State, "reason": input.Reason, "actorUserId": input.ActorUserID})
	if err != nil {
		return err
	}

	request, err := c.request(ctx, http.MethodPost, "/internal/auth/users/"+url.PathEscape(input.AuthUserID)+"/state", bytes.NewReader(body))
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient().Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("auth state update returned %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}

	return nil
}

func (c Client) GetMFAConfigured(ctx context.Context, authUserID string) (bool, error) {
	request, err := c.request(ctx, http.MethodGet, "/internal/auth/users/"+url.PathEscape(authUserID)+"/mfa-state", nil)
	if err != nil {
		return false, err
	}

	response, err := c.httpClient().Do(request)
	if err != nil {
		return false, err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return false, fmt.Errorf("auth MFA lookup returned %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}

	var out struct {
		Configured bool `json:"configured"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&out); err != nil {
		return false, err
	}

	return out.Configured, nil
}

func (c Client) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(c.Token) == "" {
		return nil, fmt.Errorf("auth administration is not configured")
	}

	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, body)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Authorization", "Bearer "+c.Token)
	return request, nil
}

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}
