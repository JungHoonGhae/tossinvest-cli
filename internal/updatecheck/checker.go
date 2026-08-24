// Package updatecheck queries GitHub Releases for the latest stable tossctl
// release and caches the result on disk so the CLI can surface "new version
// available" without hammering the API on every command.
//
// The cache TTL is 24h; network failures are silent (the goroutine just
// returns the previously cached value, or empty string on first run).
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/version"
)

const (
	defaultRepoSlug = version.Repo
	defaultInterval = 24 * time.Hour
	fetchTimeout    = 3 * time.Second
)

type cacheEntry struct {
	LastCheckedAt    time.Time `json:"last_checked_at"`
	LatestVersion    string    `json:"latest_version"`
	UpdateNotifiedAt time.Time `json:"update_notified_at,omitempty"`
	ExpiryNotifiedAt time.Time `json:"expiry_notified_at,omitempty"`
	ConfigNotifiedAt time.Time `json:"config_notified_at,omitempty"`
}

type Checker struct {
	cachePath  string
	httpClient *http.Client
	repoSlug   string
	interval   time.Duration
	now        func() time.Time
}

func New(cachePath string) *Checker {
	return &Checker{
		cachePath:  cachePath,
		httpClient: &http.Client{Timeout: fetchTimeout},
		repoSlug:   defaultRepoSlug,
		interval:   defaultInterval,
		now:        time.Now,
	}
}

// LatestStable returns the cached or freshly-fetched latest tag (no "v"
// prefix). Empty string means "no info available" — the caller should treat
// it as a no-op rather than an error.
func (c *Checker) LatestStable(ctx context.Context) string {
	return c.latest(ctx, false)
}

// LatestStableFresh ignores the cache TTL and asks the release API now, falling
// back to the cached value when the network fails.
//
// Use it for checks the user explicitly asked for (`tossctl update`). The 24h
// TTL exists to keep the *background* update notice cheap; letting it gate an
// explicit request means the command reports "already up to date" for up to a
// day after a release — while showing a "Checking latest version" spinner that
// never checked anything.
func (c *Checker) LatestStableFresh(ctx context.Context) string {
	return c.latest(ctx, true)
}

func (c *Checker) latest(ctx context.Context, force bool) string {
	entry, _ := c.readCache()
	if !force && c.now().Sub(entry.LastCheckedAt) < c.interval && entry.LatestVersion != "" {
		return entry.LatestVersion
	}

	latest, err := c.fetch(ctx)
	if err != nil || latest == "" {
		return entry.LatestVersion
	}

	entry.LastCheckedAt = c.now()
	entry.LatestVersion = latest
	_ = c.writeCache(entry)
	return latest
}

// ShouldNotifyUpdate reports whether the caller should emit an "update
// available" line for the given current version. Returns the latest version
// (empty if unknown) and a bool — true only when (a) latest is newer than
// current and (b) we haven't already notified within the cache interval. The
// caller must call MarkUpdateNotified after actually printing the notice.
func (c *Checker) ShouldNotifyUpdate(ctx context.Context, currentVersion string) (string, bool) {
	latest := c.LatestStable(ctx)
	if !IsNewer(latest, currentVersion) {
		return latest, false
	}
	entry, _ := c.readCache()
	if c.now().Sub(entry.UpdateNotifiedAt) < c.interval {
		return latest, false
	}
	return latest, true
}

func (c *Checker) MarkUpdateNotified() {
	entry, _ := c.readCache()
	entry.UpdateNotifiedAt = c.now()
	_ = c.writeCache(entry)
}

// ShouldNotifyExpiry reports whether the caller should emit a session-expiry
// warning right now. It enforces a 1-hour backoff so agents calling tossctl
// many times per minute don't see the same warning on every invocation. The
// caller must call MarkExpiryNotified after printing.
func (c *Checker) ShouldNotifyExpiry() bool {
	entry, _ := c.readCache()
	return c.now().Sub(entry.ExpiryNotifiedAt) >= time.Hour
}

func (c *Checker) MarkExpiryNotified() {
	entry, _ := c.readCache()
	entry.ExpiryNotifiedAt = c.now()
	_ = c.writeCache(entry)
}

// ShouldNotifyConfig reports whether the caller should emit a "config has
// legacy/outdated fields" warning right now. It enforces the same 24h backoff
// as the update notice so the hint doesn't repeat on every command. The caller
// must call MarkConfigNotified after printing.
func (c *Checker) ShouldNotifyConfig() bool {
	entry, _ := c.readCache()
	return c.now().Sub(entry.ConfigNotifiedAt) >= c.interval
}

func (c *Checker) MarkConfigNotified() {
	entry, _ := c.readCache()
	entry.ConfigNotifiedAt = c.now()
	_ = c.writeCache(entry)
}

func (c *Checker) fetch(ctx context.Context) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", c.repoSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var payload struct {
		TagName    string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.Prerelease {
		return "", nil
	}
	return strings.TrimPrefix(payload.TagName, "v"), nil
}

func (c *Checker) readCache() (cacheEntry, error) {
	data, err := os.ReadFile(c.cachePath)
	if err != nil {
		return cacheEntry{}, err
	}
	var e cacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return cacheEntry{}, err
	}
	return e, nil
}

func (c *Checker) writeCache(e cacheEntry) error {
	if err := os.MkdirAll(filepath.Dir(c.cachePath), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.cachePath, data, 0o600)
}

// IsNewer reports whether `latest` is semantically newer than `current`.
// Both arguments are bare semver strings without the "v" prefix. Returns
// false for empty or non-numeric inputs (e.g. dev builds) — the caller is
// expected to treat that as "skip the notice."
func IsNewer(latest, current string) bool {
	if latest == "" || current == "" || current == "dev" {
		return false
	}
	return compareSemver(latest, current) > 0
}

func compareSemver(a, b string) int {
	aParts := splitSemver(a)
	bParts := splitSemver(b)
	for i := 0; i < 3; i++ {
		av, bv := 0, 0
		if i < len(aParts) {
			av = aParts[i]
		}
		if i < len(bParts) {
			bv = bParts[i]
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}

func splitSemver(s string) []int {
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	out := make([]int, 0, 3)
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out
		}
		out = append(out, n)
	}
	return out
}
