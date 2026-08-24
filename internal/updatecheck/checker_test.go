package updatecheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"0.4.13", "0.4.12", true},
		{"0.5.0", "0.4.99", true},
		{"1.0.0", "0.9.9", true},
		{"0.4.12", "0.4.12", false},
		{"0.4.11", "0.4.12", false},
		{"0.4.12", "dev", false},
		{"", "0.4.12", false},
		{"0.4.12", "", false},
		{"0.4.13-rc1", "0.4.12", true}, // prerelease suffix stripped, compare as 0.4.13
	}
	for _, c := range cases {
		got := IsNewer(c.latest, c.current)
		if got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestLatestStableHitsCacheWithinInterval(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "update-check.json")

	// Seed cache as if we just checked 1 minute ago.
	seed := cacheEntry{LastCheckedAt: time.Now().Add(-time.Minute), LatestVersion: "9.9.9"}
	data, _ := json.Marshal(seed)
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	// HTTP server that fails the test if called — cache should win.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("HTTP server should not be hit when cache is fresh")
	}))
	defer server.Close()

	c := &Checker{
		cachePath:  cachePath,
		httpClient: server.Client(),
		repoSlug:   "x/y",
		interval:   24 * time.Hour,
		now:        time.Now,
	}

	if got := c.LatestStable(context.Background()); got != "9.9.9" {
		t.Errorf("expected cached value, got %q", got)
	}
}

func TestLatestStableRefreshesWhenStale(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "update-check.json")

	// Cache older than interval.
	seed := cacheEntry{LastCheckedAt: time.Now().Add(-48 * time.Hour), LatestVersion: "0.0.1"}
	data, _ := json.Marshal(seed)
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.4.99", "prerelease": false})
	}))
	defer server.Close()

	c := &Checker{
		cachePath:  cachePath,
		httpClient: server.Client(),
		repoSlug:   "x/y",
		interval:   24 * time.Hour,
		now:        time.Now,
	}
	// Override fetch URL via embedded test transport.
	c.httpClient = &http.Client{Transport: redirectTransport{base: server.URL}}

	if got := c.LatestStable(context.Background()); got != "0.4.99" {
		t.Errorf("expected refreshed value, got %q", got)
	}

	// Cache should be updated.
	var refreshed cacheEntry
	raw, _ := os.ReadFile(cachePath)
	_ = json.Unmarshal(raw, &refreshed)
	if refreshed.LatestVersion != "0.4.99" {
		t.Errorf("cache not updated, got %q", refreshed.LatestVersion)
	}
}

func TestLatestStableNetworkFailureReturnsCachedValue(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "update-check.json")
	seed := cacheEntry{LastCheckedAt: time.Now().Add(-48 * time.Hour), LatestVersion: "0.4.10"}
	data, _ := json.Marshal(seed)
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	c := &Checker{
		cachePath:  cachePath,
		httpClient: &http.Client{Transport: failingTransport{}},
		repoSlug:   "x/y",
		interval:   24 * time.Hour,
		now:        time.Now,
	}
	if got := c.LatestStable(context.Background()); got != "0.4.10" {
		t.Errorf("expected stale cached value on network failure, got %q", got)
	}
}

// redirectTransport rewrites all requests to point at the test server.
type redirectTransport struct{ base string }

func (rt redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target, err := http.NewRequest(req.Method, rt.base, nil)
	if err != nil {
		return nil, err
	}
	target.Header = req.Header
	return http.DefaultTransport.RoundTrip(target)
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, http.ErrHandlerTimeout
}

func TestShouldNotifyUpdateBackoff(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "update-check.json")
	seed := cacheEntry{LastCheckedAt: time.Now().Add(-time.Minute), LatestVersion: "0.4.13"}
	data, _ := json.Marshal(seed)
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	c := &Checker{cachePath: cachePath, repoSlug: "x/y", interval: 24 * time.Hour, now: time.Now}

	// First call: should notify.
	latest, ok := c.ShouldNotifyUpdate(context.Background(), "0.4.12")
	if !ok || latest != "0.4.13" {
		t.Fatalf("first call: expected notify=true latest=0.4.13, got notify=%v latest=%q", ok, latest)
	}
	c.MarkUpdateNotified()

	// Second call within the interval: should suppress.
	if _, ok := c.ShouldNotifyUpdate(context.Background(), "0.4.12"); ok {
		t.Fatal("second call within interval: expected notify=false (suppressed by backoff)")
	}

	// Same version: should not notify.
	if _, ok := c.ShouldNotifyUpdate(context.Background(), "0.4.13"); ok {
		t.Fatal("same version: expected notify=false")
	}
}

func TestShouldNotifyExpiryHourlyBackoff(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "update-check.json")
	c := &Checker{cachePath: cachePath, repoSlug: "x/y", interval: 24 * time.Hour, now: time.Now}

	// No cache yet: should notify.
	if !c.ShouldNotifyExpiry() {
		t.Fatal("first call: expected notify=true")
	}
	c.MarkExpiryNotified()

	// Immediate re-check: suppressed.
	if c.ShouldNotifyExpiry() {
		t.Fatal("immediate re-check: expected notify=false (suppressed within hour)")
	}

	// Backdate the cache by 2h and re-check: should notify again.
	entry, _ := c.readCache()
	entry.ExpiryNotifiedAt = time.Now().Add(-2 * time.Hour)
	if err := c.writeCache(entry); err != nil {
		t.Fatal(err)
	}
	if !c.ShouldNotifyExpiry() {
		t.Fatal("after 2h: expected notify=true")
	}
}

// `tossctl update` 는 사용자가 명시적으로 "지금 갱신해" 라고 부른 것이다. 24시간
// 캐시는 **백그라운드 알림**을 싸게 만들려고 있는 것이지, 명시적 요청을 막으라고
// 있는 게 아니다. 캐시가 이걸 가로막으면 릴리즈 직후 최대 하루 동안 "이미 최신"
// 이라고 답한다 — 스피너까지 돌리면서.
func TestLatestStableFreshIgnoresCacheTTL(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "update-check.json")

	// 1분 전에 확인했고 그때는 0.40.0 이 최신이었다.
	seed := cacheEntry{LastCheckedAt: time.Now().Add(-time.Minute), LatestVersion: "0.40.0"}
	data, _ := json.Marshal(seed)
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	// 그 사이 0.42.0 이 나왔다.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.42.0", "prerelease": false})
	}))
	defer server.Close()

	c := &Checker{
		cachePath:  cachePath,
		httpClient: &http.Client{Transport: redirectTransport{base: server.URL}},
		repoSlug:   "x/y",
		interval:   24 * time.Hour,
		now:        time.Now,
	}

	// 배경 경로는 캐시를 그대로 쓴다 — 이건 의도된 동작이다.
	if got := c.LatestStable(context.Background()); got != "0.40.0" {
		t.Errorf("background path should stay cached, got %q", got)
	}
	// 명시적 경로는 지금 확인해야 한다.
	if got := c.LatestStableFresh(context.Background()); got != "0.42.0" {
		t.Errorf("explicit check must ignore the TTL, got %q", got)
	}
	// 그리고 그 결과가 캐시에 반영돼 배경 경로도 따라와야 한다.
	if got := c.LatestStable(context.Background()); got != "0.42.0" {
		t.Errorf("a fresh check should update the cache, got %q", got)
	}
}

// 네트워크가 죽었을 때 강제 확인이 빈 값을 돌려주면 update 커맨드가 "최신 버전을
// 알 수 없음" 으로 오작동한다. 캐시된 값으로 물러나야 한다.
func TestLatestStableFreshFallsBackToCacheOffline(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "update-check.json")
	seed := cacheEntry{LastCheckedAt: time.Now().Add(-time.Minute), LatestVersion: "0.40.0"}
	data, _ := json.Marshal(seed)
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := &Checker{
		cachePath:  cachePath,
		httpClient: &http.Client{Transport: redirectTransport{base: server.URL}},
		repoSlug:   "x/y",
		interval:   24 * time.Hour,
		now:        time.Now,
	}
	if got := c.LatestStableFresh(context.Background()); got != "0.40.0" {
		t.Errorf("offline forced check must fall back to cache, got %q", got)
	}
}
