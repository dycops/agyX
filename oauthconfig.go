package main

// OAuth client configuration for the Google Authorization Code flow that mints
// agy credentials. The client_id is public (it appears in the browser URL
// during login). The client_secret is the Antigravity app's credential — the
// same for every installation, non-personal — kept in a local oauth.json so it
// never enters a chat transcript and can be carried to a new machine.

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Public client id, extracted from Antigravity Manager. Safe to embed.
const defaultClientID = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"

var oauthScopes = []string{
	"https://www.googleapis.com/auth/aicode",
	"https://www.googleapis.com/auth/cclog",
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/experimentsandconfigs",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
}

type OAuthConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	AuthMethod   string `json:"auth_method"`
}

func oauthConfigPath() string {
	return filepath.Join(vaultDir(), "oauth.json")
}

func loadOAuthConfig() (OAuthConfig, error) {
	cfg := OAuthConfig{ClientID: defaultClientID}
	data, err := os.ReadFile(oauthConfigPath())
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.ClientID == "" {
		cfg.ClientID = defaultClientID
	}
	return cfg, nil
}

func saveOAuthConfig(cfg OAuthConfig) error {
	if err := os.MkdirAll(vaultDir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(oauthConfigPath(), data, 0o600)
}
