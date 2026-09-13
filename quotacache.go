package main

// Quota cache: ~/.agyx/quota.json, one entry per account email with
// the fetch time. Holds only percentages and reset timestamps — no tokens.
// Entries younger than quotaTTL are served without touching the backend, so
// relaunching agyx a few times in a row doesn't re-query Google.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type cachedGroup struct {
	Pct        int    `json:"pct"`
	FiveHPct   int    `json:"pct_5h"`
	WeekReset  string `json:"week_reset,omitempty"`
	FiveHReset string `json:"reset_5h,omitempty"`
}

type cachedQuota struct {
	At  time.Time   `json:"at"`
	Gem cachedGroup `json:"gemini"`
	Cla cachedGroup `json:"claude"`
}

func quotaCachePath() string {
	return filepath.Join(vaultDir(), "quota.json")
}

func toCached(g groupQuota) cachedGroup {
	return cachedGroup{Pct: g.pct, FiveHPct: g.fiveHPct, WeekReset: g.weekReset, FiveHReset: g.fiveHReset}
}

func fromCached(c cachedGroup) groupQuota {
	return groupQuota{pct: c.Pct, fiveHPct: c.FiveHPct, weekReset: c.WeekReset, fiveHReset: c.FiveHReset}
}

// loadQuotaCache returns the cache map; a missing or unreadable file is an
// empty cache, never an error — quota is best-effort.
func loadQuotaCache() map[string]cachedQuota {
	m := map[string]cachedQuota{}
	data, err := os.ReadFile(quotaCachePath())
	if err != nil {
		return m
	}
	if json.Unmarshal(data, &m) != nil {
		return map[string]cachedQuota{}
	}
	return m
}

func saveQuotaCache(m map[string]cachedQuota) {
	if err := os.MkdirAll(vaultDir(), 0o700); err != nil {
		return
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(quotaCachePath(), data, 0o600)
}

// cachedQuotaInfo returns the cache entry for email if it is younger than ttl.
func cachedQuotaInfo(cache map[string]cachedQuota, email string, ttl time.Duration) (quotaInfo, bool) {
	c, ok := cache[email]
	if !ok || time.Since(c.At) > ttl {
		return quotaInfo{}, false
	}
	return quotaInfo{gem: fromCached(c.Gem), cla: fromCached(c.Cla)}, true
}

// known reports whether a fetched quota carries any real data (worth caching).
func (q quotaInfo) known() bool {
	return q.gem.pct >= 0 || q.gem.fiveHPct >= 0 || q.cla.pct >= 0 || q.cla.fiveHPct >= 0
}
