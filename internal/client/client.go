package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/jackzhao/robinhood-cli/internal/auth"
	"github.com/jackzhao/robinhood-cli/internal/config"
)

type Client struct {
	http  *http.Client
	mu    sync.Mutex
	creds *auth.Credentials
}

func New() (*Client, error) {
	creds, err := auth.Load()
	if err != nil {
		return nil, err
	}
	return &Client{
		http:  &http.Client{Timeout: 30 * time.Second},
		creds: creds,
	}, nil
}

func (c *Client) ensureFresh() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.creds.Expired() {
		return nil
	}
	refreshed, err := auth.Refresh(c.creds)
	if err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}
	c.creds = refreshed
	return nil
}

// GetJSON performs an authenticated GET and decodes JSON into dest.
// fullURL is an absolute https://api.robinhood.com/... URL.
func (c *Client) GetJSON(fullURL string, dest any) error {
	if err := c.ensureFresh(); err != nil {
		return err
	}
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return err
	}
	c.applyHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized (%s) — try `rh login` again", fullURL)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GET %s: http %d: %s", fullURL, resp.StatusCode, string(body))
	}
	if dest == nil {
		return nil
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode %s: %w (body=%s)", fullURL, err, truncate(body, 300))
	}
	return nil
}

// PostJSON performs an authenticated POST with a JSON payload.
// Returns the HTTP status code and decodes the response body into dest (if non-nil).
// Unlike GetJSON, this does NOT treat 4xx as a Go error — the caller decides,
// because trade endpoints return structured validation errors that the caller
// wants to surface to the user verbatim.
func (c *Client) PostJSON(fullURL string, payload, dest any) (int, error) {
	if err := c.ensureFresh(); err != nil {
		return 0, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequest("POST", fullURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	if dest != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return resp.StatusCode, fmt.Errorf("decode %s: %w (body=%s)", fullURL, err, truncate(respBody, 500))
		}
	}
	return resp.StatusCode, nil
}

// GetJSONUnauth is for endpoints that work without a bearer (instruments, chains).
func GetJSONUnauth(fullURL string, dest any) error {
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", config.UserAgent)

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GET %s: http %d: %s", fullURL, resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, dest)
}

func (c *Client) applyHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("X-Robinhood-API-Version", "1.431.4")
	req.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)
}

// Helper to build api.robinhood.com URLs with query params.
func URL(path string, query map[string]string) string {
	u := config.APIBase + path
	if len(query) == 0 {
		return u
	}
	v := url.Values{}
	for k, val := range query {
		if val != "" {
			v.Set(k, val)
		}
	}
	return u + "?" + v.Encode()
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...(truncated)"
}
