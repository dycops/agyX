package main

// Quota lookup against the Cloud Code backend, mirroring Antigravity Manager:
// identify as the Antigravity client via User-Agent, resolve the account's
// project through loadCodeAssist, then read retrieveUserQuotaSummary. The
// response groups models (Gemini / Claude+GPT); each group has a weekly and a
// 5-hour bucket with a remainingFraction; the TUI shows both windows for
// both groups.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	quotaURL = "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary"
	loadURL  = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"

	// The server gates quota on this product token; Antigravity reports itself
	// as `antigravity/<version>`.
	antigravityUA = "antigravity/2.0.3"
)

type quotaResp struct {
	Groups []struct {
		DisplayName string `json:"displayName"`
		Buckets     []struct {
			Window            string  `json:"window"`
			RemainingFraction float64 `json:"remainingFraction"`
			ResetTime         string  `json:"resetTime"`
		} `json:"buckets"`
	} `json:"groups"`
}

func tokenSource(rawCred string) (oauth2.TokenSource, error) {
	var c credential
	if err := json.Unmarshal([]byte(rawCred), &c); err != nil {
		return nil, err
	}
	if c.Token == nil {
		return nil, fmt.Errorf("в креде нет token")
	}
	cfg, err := loadOAuthConfig()
	if err != nil {
		return nil, err
	}
	oc := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       oauthScopes,
	}
	return oc.TokenSource(context.Background(), c.Token), nil
}

func callCodeAssist(ts oauth2.TokenSource, url string, body any) (int, []byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	tok, err := ts.Token()
	if err != nil {
		return 0, nil, fmt.Errorf("refresh токена: %w", err)
	}
	tok.SetAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", antigravityUA)

	resp, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, data, nil
}

func fetchProject(ts oauth2.TokenSource) string {
	st, body, err := callCodeAssist(ts, loadURL, map[string]any{
		"metadata": map[string]any{"ideType": "ANTIGRAVITY"},
	})
	if err != nil || st != http.StatusOK {
		return ""
	}
	var r struct {
		CloudaicompanionProject string `json:"cloudaicompanionProject"`
	}
	_ = json.Unmarshal(body, &r)
	return r.CloudaicompanionProject
}

func fetchQuota(ts oauth2.TokenSource) (quotaResp, error) {
	var r quotaResp
	st, data, err := callCodeAssist(ts, quotaURL, map[string]any{"project": fetchProject(ts)})
	if err != nil {
		return r, err
	}
	if st == http.StatusForbidden {
		st, data, err = callCodeAssist(ts, quotaURL, map[string]any{})
		if err != nil {
			return r, err
		}
	}
	if st != http.StatusOK {
		return r, fmt.Errorf("quota http %d", st)
	}
	err = json.Unmarshal(data, &r)
	return r, err
}

// groupQuota is one model group's weekly remaining percent and the reset
// timestamps (RFC3339) of its weekly and 5-hour buckets.
type groupQuota struct {
	pct        int // weekly remaining %, -1 if unknown
	fiveHPct   int // 5-hour window remaining %, -1 if unknown
	weekReset  string
	fiveHReset string
}

func unknownGroup() groupQuota { return groupQuota{pct: -1, fiveHPct: -1} }

// group returns the quota of the first group whose lowercased display name
// satisfies match.
func (q quotaResp) group(match func(name string) bool) groupQuota {
	g := unknownGroup()
	for _, grp := range q.Groups {
		if !match(strings.ToLower(grp.DisplayName)) {
			continue
		}
		for _, b := range grp.Buckets {
			switch b.Window {
			case "weekly":
				g.pct = int(b.RemainingFraction * 100)
				g.weekReset = b.ResetTime
			case "5h":
				g.fiveHPct = int(b.RemainingFraction * 100)
				g.fiveHReset = b.ResetTime
			}
		}
		break
	}
	return g
}

// quotaInfo holds both model groups' quota.
type quotaInfo struct {
	gem groupQuota // Gemini models
	cla groupQuota // Claude and GPT models
}

func quotaInfoOf(rawCred string) quotaInfo {
	qi := quotaInfo{gem: unknownGroup(), cla: unknownGroup()}
	ts, err := tokenSource(rawCred)
	if err != nil {
		return qi
	}
	q, err := fetchQuota(ts)
	if err != nil {
		return qi
	}
	qi.gem = q.group(func(n string) bool { return strings.Contains(n, "gemini") })
	qi.cla = q.group(func(n string) bool {
		return strings.Contains(n, "claude") || strings.Contains(n, "gpt")
	})
	return qi
}

// prefetchQuotas returns quota for all accounts: fresh cache entries as-is,
// the rest fetched concurrently, capped so the TUI never hangs. Accounts that
// don't finish in time stay unknown (-1/-1) and are not cached. force skips
// the cache (manual refresh).
func prefetchQuotas(accts []Account, force bool) map[string]quotaInfo {
	cache := loadQuotaCache()
	ttl := quotaTTL()
	if force {
		ttl = 0 // every entry counts as stale
	}
	out := make(map[string]quotaInfo, len(accts))
	var stale []Account
	for _, a := range accts {
		if qi, ok := cachedQuotaInfo(cache, a.Email, ttl); ok {
			out[a.Email] = qi
			continue
		}
		out[a.Email] = quotaInfo{gem: unknownGroup(), cla: unknownGroup()}
		stale = append(stale, a)
	}
	if len(stale) == 0 {
		return out
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, a := range stale {
		wg.Add(1)
		go func(a Account) {
			defer wg.Done()
			qi := quotaInfoOf(a.Token)
			mu.Lock()
			out[a.Email] = qi
			mu.Unlock()
		}(a)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
	}

	mu.Lock()
	defer mu.Unlock()
	now := time.Now()
	for _, a := range stale {
		if v := out[a.Email]; v.known() {
			cache[a.Email] = cachedQuota{At: now, Gem: toCached(v.gem), Cla: toCached(v.cla)}
		}
	}
	saveQuotaCache(cache)
	res := make(map[string]quotaInfo, len(out))
	for k, v := range out {
		res[k] = v
	}
	return res
}

// cmdQuota prints the resolved quota for the live account (verification).
func cmdQuota() error {
	raw, err := readLive()
	if err != nil {
		return err
	}
	ts, err := tokenSource(raw)
	if err != nil {
		return err
	}
	q, err := fetchQuota(ts)
	if err != nil {
		return err
	}
	for _, g := range q.Groups {
		fmt.Printf("%s\n", g.DisplayName)
		for _, b := range g.Buckets {
			fmt.Printf("  %-6s %3d%%  reset=%s\n", b.Window, int(b.RemainingFraction*100), b.ResetTime)
		}
	}
	return nil
}
