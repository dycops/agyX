package main

// resolveEmail derives a human label for an account. The credential itself
// carries no email, so we use its freshly-issued Google access token to call
// Google's userinfo endpoint (all managed accounts are provider=google). This
// runs locally in the tool; the token is sent only to Google over TLS.

import (
	"encoding/json"
	"net/http"
	"time"
)

type liveToken struct {
	Token struct {
		AccessToken string `json:"access_token"`
	} `json:"token"`
}

func accessTokenOf(raw string) string {
	var lt liveToken
	if json.Unmarshal([]byte(raw), &lt) != nil {
		return ""
	}
	return lt.Token.AccessToken
}

// authMethodOf pulls the non-secret "auth_method" label out of a credential.
func authMethodOf(raw string) string {
	var v struct {
		AuthMethod string `json:"auth_method"`
	}
	if json.Unmarshal([]byte(raw), &v) != nil {
		return ""
	}
	return v.AuthMethod
}

// refreshTokenOf returns the credential's refresh_token — a stable per-account
// identity that survives access-token refreshes, used to tell which account is
// currently live.
func refreshTokenOf(raw string) string {
	var v struct {
		Token struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"token"`
	}
	if json.Unmarshal([]byte(raw), &v) != nil {
		return ""
	}
	return v.Token.RefreshToken
}

// sameAccount reports whether two credentials belong to the same account.
func sameAccount(a, b string) bool {
	ra, rb := refreshTokenOf(a), refreshTokenOf(b)
	return ra != "" && ra == rb
}

func resolveEmail(rawCredential string) string {
	at := accessTokenOf(rawCredential)
	if at == "" {
		return ""
	}
	req, err := http.NewRequest(http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+at)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var info struct {
		Email string `json:"email"`
	}
	if json.NewDecoder(resp.Body).Decode(&info) != nil {
		return ""
	}
	return info.Email
}
