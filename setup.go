package main

// `agyx setup-oauth` — one-time extraction of the OAuth client credentials into
// ~/.agyx/oauth.json. The client_secret is Antigravity's app credential,
// embedded identically in every Antigravity binary. We read it straight out of
// agy.exe (always present, since we wrap it), falling back to the Antigravity
// IDE or the Manager. The secret is written to disk only — never printed.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var reSecret = regexp.MustCompile(`GOCSPX-[A-Za-z0-9_-]{10,}`)

// secretCandidates lists files that embed the Antigravity OAuth client secret,
// most-reliable first.
func secretCandidates() []string {
	la := os.Getenv("LOCALAPPDATA")
	pf86 := os.Getenv("ProgramFiles(x86)")
	pf := os.Getenv("ProgramFiles")

	var c []string
	add := func(p string) {
		if p != "" {
			c = append(c, p)
		}
	}
	// Manager first: it embeds exactly the 1071 client, so its secret is the
	// correct, unambiguous pair. agy.exe carries a different client's secret.
	for _, base := range []string{pf86, pf} {
		if base == "" {
			continue
		}
		m, _ := filepath.Glob(filepath.Join(base, "Antigravity Manager", "app-*", "resources", "app.asar"))
		c = append(c, m...)
	}
	// Antigravity IDE bundles.
	add(filepath.Join(la, "Programs", "Antigravity", "resources", "app.asar"))
	add(filepath.Join(la, "Programs", "Antigravity IDE", "resources", "app.asar"))
	// agy itself — last resort (its embedded secret belongs to another client).
	add(filepath.Join(la, "agy", "bin", "agy.exe"))
	return c
}

// extractOAuthCreds returns the client secret paired with the known Antigravity
// client id. The binaries embed several Google clients, so we locate our exact
// client id and take the GOCSPX secret nearest to it (falling back to any secret
// in the same file). The client id is always the known-good one.
func extractOAuthCreds() (secret, src string, err error) {
	target := []byte(defaultClientID)
	for _, p := range secretCandidates() {
		data, e := os.ReadFile(p)
		if e != nil {
			continue
		}
		idx := bytes.Index(data, target)
		if idx < 0 {
			continue // this file doesn't carry our client — skip it
		}
		lo, hi := idx-6000, idx+6000
		if lo < 0 {
			lo = 0
		}
		if hi > len(data) {
			hi = len(data)
		}
		s := reSecret.Find(data[lo:hi])
		if s == nil {
			s = reSecret.Find(data) // fallback: only secret in this file
		}
		if s == nil {
			continue
		}
		return string(s), p, nil
	}
	return "", "", errors.New(T("no_secret_found"))
}

func cmdSetupOAuth() error {
	secret, src, err := extractOAuthCreds()
	if err != nil {
		return err
	}
	cfg := OAuthConfig{ClientID: defaultClientID, ClientSecret: secret}
	// Reuse the auth_method label from the current live credential, if present.
	if live, e := readLive(); e == nil {
		cfg.AuthMethod = authMethodOf(live)
	}
	if err := saveOAuthConfig(cfg); err != nil {
		return err
	}
	fmt.Printf(T("setup_written"), oauthConfigPath())
	fmt.Printf(T("setup_source"), src)
	fmt.Printf(T("setup_id"), cfg.ClientID)
	fmt.Printf(T("setup_secret"), len(cfg.ClientSecret))
	fmt.Printf(T("setup_method"), cfg.AuthMethod)
	return nil
}
