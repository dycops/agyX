package main

// `agyx add` — logs a new Google account in through the browser (the same
// Authorization Code flow the Antigravity Manager uses), builds the credential
// in agy's exact format, writes it to the credential store, and saves it to the
// vault. No Manager required.

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"os/exec"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// The credential agy reads: an oauth2.Token under "token", plus a mode label.
// oauth2.Token marshals to {access_token, token_type, refresh_token, expiry} —
// identical to what agy (same library) writes.
type credential struct {
	Token      *oauth2.Token `json:"token"`
	AuthMethod string        `json:"auth_method"`
}

func cmdAdd(label string) error {
	cfg, err := loadOAuthConfig()
	if err != nil {
		return err
	}
	if cfg.ClientSecret == "" {
		return errors.New(T("no_secret"))
	}
	authMethod := cfg.AuthMethod
	if authMethod == "" {
		if live, e := readLive(); e == nil {
			authMethod = authMethodOf(live)
		}
	}

	// Loopback receiver on a free port (Desktop client → any loopback port).
	// Advertise the literal address we listen on, so the browser can't resolve
	// "localhost" to ::1 and miss us.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	redirect := fmt.Sprintf("http://127.0.0.1:%d/", port)

	oc := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  redirect,
		Scopes:       oauthScopes,
	}
	state := randHex(16)
	authURL := oc.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "select_account consent"),
	)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// Anything on this port that doesn't carry our state is noise (a stray
		// favicon request, another local process, a web page probing loopback):
		// reject it without touching the flow. Google echoes state on error
		// redirects too, so the check comes first.
		if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if e := q.Get("error"); e != "" {
			fmt.Fprintf(w, T("add_login_err"), html.EscapeString(e))
			select {
			case errCh <- fmt.Errorf("oauth error: %s", e):
			default:
			}
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, T("add_done_html"))
		select {
		case codeCh <- code: // first code wins; later duplicates are dropped
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln)
	defer srv.Close()

	fmt.Println(T("add_opening"))
	fmt.Println(T("add_manual"))
	fmt.Println("  " + authURL)
	openBrowser(authURL)

	var code string
	select {
	case code = <-codeCh:
	case err = <-errCh:
		return err
	case <-time.After(5 * time.Minute):
		return errors.New(T("add_timeout"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tok, err := oc.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf(T("exchange_fail"), err)
	}

	cred := credential{Token: tok, AuthMethod: authMethod}
	raw, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	if err := writeLive(string(raw)); err != nil {
		return fmt.Errorf(T("write_store"), err)
	}

	// Snapshot into the vault under a resolved label.
	email := label
	if email == "" {
		email = resolveEmail(string(raw))
	}
	if email == "" {
		email = fmt.Sprintf("account-%d", time.Now().Unix())
	}
	accts, _ := loadVault()
	accts = upsert(accts, Account{Email: email, Token: string(raw), AddedAt: time.Now()})
	if err := saveVault(accts); err != nil {
		return err
	}
	fmt.Printf(T("added_active"), email)
	return nil
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func openBrowser(u string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
}
