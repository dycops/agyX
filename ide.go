package main

// Antigravity IDE account switch. The IDE keeps its own copy of the OAuth
// token in its VS Code state database:
//   %APPDATA%\Antigravity IDE\User\globalStorage\state.vscdb
//   ItemTable: key = antigravityUnifiedStateSync.oauthToken
// The value is a base64 "unified topic entry" — a hand-rolled protobuf:
//   topic_entry(1) {
//     key(1)  = "oauthTokenInfoSentinelKey"
//     row(2)  { value(1) = base64(oauth_info) }
//   }
//   oauth_info { access_token(1), refresh_token(3), expiry(4){seconds(1)}, id_token(5) }
// userStatus uses the same envelope with { email(3), email(7) }.
// This mirrors what Antigravity Manager writes on switch. The IDE must not be
// running: it holds the database and rewrites it on exit.

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	ideKeyOAuth      = "antigravityUnifiedStateSync.oauthToken"
	ideKeyUserStatus = "antigravityUnifiedStateSync.userStatus"
	ideKeyInitState  = "jetskiStateSync.agentManagerInitState"

	ideOAuthSentinel      = "oauthTokenInfoSentinelKey"
	ideUserStatusSentinel = "userStatusSentinelKey"
)

func ideStatePath() string {
	return filepath.Join(os.Getenv("APPDATA"), "Antigravity IDE", "User", "globalStorage", "state.vscdb")
}

// --- minimal protobuf wire encoding ---------------------------------------

func pbVarint(v uint64) []byte {
	var b []byte
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func pbBytes(field int, data []byte) []byte {
	out := pbVarint(uint64(field)<<3 | 2)
	out = append(out, pbVarint(uint64(len(data)))...)
	return append(out, data...)
}

func pbString(field int, s string) []byte { return pbBytes(field, []byte(s)) }

func pbVarintField(field int, v uint64) []byte {
	return append(pbVarint(uint64(field)<<3), pbVarint(v)...)
}

// unifiedEntry wraps a payload the way the IDE stores it (see file comment).
func unifiedEntry(sentinel string, payload []byte) string {
	row := pbString(1, base64.StdEncoding.EncodeToString(payload))
	inner := append(pbString(1, sentinel), pbBytes(2, row)...)
	return base64.StdEncoding.EncodeToString(pbBytes(1, inner))
}

// ideOAuthPayload builds oauth_info from an agy credential JSON.
func ideOAuthPayload(rawCred string) ([]byte, error) {
	var c credential
	if err := json.Unmarshal([]byte(rawCred), &c); err != nil {
		return nil, err
	}
	if c.Token == nil || c.Token.AccessToken == "" || c.Token.RefreshToken == "" {
		return nil, errors.New("credential has no access/refresh token")
	}
	var p []byte
	p = append(p, pbString(1, c.Token.AccessToken)...)
	p = append(p, pbString(3, c.Token.RefreshToken)...)
	exp := c.Token.Expiry
	if exp.IsZero() {
		exp = time.Now().Add(-time.Minute) // expired → IDE refreshes on start
	}
	p = append(p, pbBytes(4, pbVarintField(1, uint64(exp.Unix())))...)
	if id, ok := c.Token.Extra("id_token").(string); ok && id != "" {
		p = append(p, pbString(5, id)...)
	}
	return p, nil
}

// ideRunning reports whether the IDE process is alive.
func ideRunning() bool {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq Antigravity IDE.exe", "/NH").Output()
	if err != nil {
		return false
	}
	// tasklist prints the matching row, or an "INFO: No tasks..." notice
	// (localized), never both — a row always carries the image name.
	return strings.Contains(strings.ToLower(string(out)), "antigravity ide.exe")
}

// switchIDE writes the account's token into the IDE state database. A
// one-time backup sits next to the database. Returns a human-readable note.
func switchIDE(email, rawCred string) (string, error) {
	p := ideStatePath()
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("IDE state not found: %s", p)
	}
	if ideRunning() {
		return "", errors.New(T("ide_running"))
	}
	payload, err := ideOAuthPayload(rawCred)
	if err != nil {
		return "", err
	}
	if bak := p + ".agyx.bak"; !fileExists(bak) {
		if data, err := os.ReadFile(p); err == nil {
			_ = os.WriteFile(bak, data, 0o600)
		}
	}

	db, err := sql.Open("sqlite", p+"?_busy_timeout=3000")
	if err != nil {
		return "", err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	upsert := "INSERT OR REPLACE INTO ItemTable (key, value) VALUES (?, ?)"
	if _, err := tx.ExecContext(ctx, upsert, ideKeyOAuth, unifiedEntry(ideOAuthSentinel, payload)); err != nil {
		return "", err
	}
	status := append(pbString(3, email), pbString(7, email)...)
	if _, err := tx.ExecContext(ctx, upsert, ideKeyUserStatus, unifiedEntry(ideUserStatusSentinel, status)); err != nil {
		return "", err
	}
	// Force the agent manager to re-initialise under the new identity.
	if _, err := tx.ExecContext(ctx, "DELETE FROM ItemTable WHERE key = ?", ideKeyInitState); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return fmt.Sprintf(T("ide_switched"), email), nil
}

// cmdIDE: `agyx ide` — push the currently live account into the IDE.
func cmdIDE() error {
	live, err := readLive()
	if err != nil {
		return err
	}
	email := ""
	if accts, _ := loadVault(); accts != nil {
		for _, a := range accts {
			if sameAccount(a.Token, live) {
				email = a.Email
				break
			}
		}
	}
	if email == "" {
		email = resolveEmail(live)
	}
	if email == "" {
		return errors.New(T("active_unknown"))
	}
	note, err := switchIDE(email, live)
	if err != nil {
		return err
	}
	fmt.Println(note)
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
