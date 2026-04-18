package config

import (
	"os"
	"path/filepath"
)

const (
	APIBase   = "https://api.robinhood.com"
	NummusAPI = "https://nummus.robinhood.com"

	// Public Robinhood iOS/Android client_id used by official apps.
	ClientID = "c82SH0WZOsabOXGP2sxqcj34FxkvfnWRZBKlBjFS"

	UserAgent = "Robinhood/8232 (com.robinhood.release.Robinhood; build:8232; iOS 17.5.1)"
)

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".robinhood-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func CredentialsPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

func DeviceTokenPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "device_token"), nil
}
