package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/jackzhao/robinhood-cli/internal/config"
)

type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	DeviceToken  string    `json:"device_token"`
}

func (c *Credentials) Expired() bool {
	return time.Now().After(c.ExpiresAt.Add(-60 * time.Second))
}

func Load() (*Credentials, error) {
	path, err := config.CredentialsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not logged in: run `rh login` first")
		}
		return nil, err
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid credentials file: %w", err)
	}
	return &c, nil
}

func Save(c *Credentials) error {
	path, err := config.CredentialsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func Clear() error {
	path, err := config.CredentialsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// LoadOrCreateDeviceToken keeps a stable device_token across sessions.
// Robinhood ties verification approvals to this token, so generating a
// fresh one each login would force the user to re-verify every time.
func LoadOrCreateDeviceToken() (string, error) {
	path, err := config.DeviceTokenPath()
	if err != nil {
		return "", err
	}
	if data, err := os.ReadFile(path); err == nil {
		token := string(data)
		if len(token) > 0 {
			return token, nil
		}
	}
	token := uuid.NewString()
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		return "", err
	}
	return token, nil
}
