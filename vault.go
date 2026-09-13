package main

// The vault: a DPAPI-encrypted JSON blob holding one saved credential per
// account. The account token is treated as an opaque string — we never parse
// or transform its secret parts.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Account struct {
	Email   string    `json:"email"` // label; "" if it could not be derived
	Token   string    `json:"token"` // opaque credential-store value
	AddedAt time.Time `json:"added_at"`
}

// vaultDir is ~/.agyx. An older ~/.agy-switcher is moved over once.
func vaultDir() string {
	home := os.Getenv("USERPROFILE")
	dir := filepath.Join(home, ".agyx")
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		if old := filepath.Join(home, ".agy-switcher"); fileExists(old) {
			_ = os.Rename(old, dir)
		}
	}
	return dir
}

func vaultPath() string {
	return filepath.Join(vaultDir(), "vault.bin")
}

func loadVault() ([]Account, error) {
	enc, err := os.ReadFile(vaultPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(enc) == 0 {
		return nil, nil
	}
	plain, err := dpapiUnprotect(enc)
	if err != nil {
		return nil, err
	}
	var accts []Account
	if err := json.Unmarshal(plain, &accts); err != nil {
		return nil, err
	}
	return accts, nil
}

func saveVault(accts []Account) error {
	if err := os.MkdirAll(vaultDir(), 0o700); err != nil {
		return err
	}
	plain, err := json.Marshal(accts)
	if err != nil {
		return err
	}
	enc, err := dpapiProtect(plain)
	if err != nil {
		return err
	}
	// Write via a temp file + rename so a crash never leaves a half-written vault.
	tmp := vaultPath() + ".tmp"
	if err := os.WriteFile(tmp, enc, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, vaultPath())
}

func upsert(accts []Account, a Account) []Account {
	key := a.Email
	if key == "" {
		// No email: treat every anonymous import as a new slot.
		return append(accts, a)
	}
	for i := range accts {
		if strings.EqualFold(accts[i].Email, key) {
			accts[i].Token = a.Token
			accts[i].AddedAt = a.AddedAt
			return accts
		}
	}
	return append(accts, a)
}
