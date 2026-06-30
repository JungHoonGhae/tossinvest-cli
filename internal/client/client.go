package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/junghoonkye/tossinvest-cli/internal/config"
	"github.com/junghoonkye/tossinvest-cli/internal/session"
)

const defaultAPIBaseURL = "https://wts-api.tossinvest.com"
const defaultInfoBaseURL = "https://wts-info-api.tossinvest.com"
const defaultCertBaseURL = "https://wts-cert-api.tossinvest.com"

// defaultUserAgent is sent on every request to avoid being blocked by the
// Toss Securities edge, which rejects Go's default "Go-http-client/1.1" UA.
const defaultUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36"

type Config struct {
	HTTPClient    *http.Client
	APIBaseURL    string
	InfoBaseURL   string
	CertBaseURL   string
	Session       *session.Session
	TradingPolicy config.Trading
}

type Client struct {
	httpClient    *http.Client
	apiBaseURL    string
	infoBaseURL   string
	certBaseURL   string
	session       *session.Session
	tradingPolicy config.Trading
	browserTabID  string
	appVersion    string
}

func New(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	apiBaseURL := strings.TrimRight(cfg.APIBaseURL, "/")
	infoBaseURL := strings.TrimRight(cfg.InfoBaseURL, "/")
	certBaseURL := strings.TrimRight(cfg.CertBaseURL, "/")
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBaseURL
	}
	if infoBaseURL == "" {
		infoBaseURL = defaultInfoBaseURL
	}
	if certBaseURL == "" {
		certBaseURL = defaultCertBaseURL
	}

	return &Client{
		httpClient:    httpClient,
		apiBaseURL:    apiBaseURL,
		infoBaseURL:   infoBaseURL,
		certBaseURL:   certBaseURL,
		session:       cfg.Session,
		tradingPolicy: cfg.TradingPolicy,
		browserTabID:  inferBrowserTabID(cfg.Session),
		appVersion:    inferAppVersion(cfg.Session),
	}
}

var (
	mainBundlePattern = regexp.MustCompile(`/assets/v2/_next/static/chunks/main-[^"]+\.js`)
	appVersionPattern = regexp.MustCompile(`v\d{6}\.\d{4}`)
)

func inferBrowserTabID(sess *session.Session) string {
	if sess == nil {
		return newBrowserTabID()
	}
	if value := strings.TrimSpace(sess.Headers["Browser-Tab-Id"]); value != "" {
		return value
	}
	if value := strings.TrimSpace(sess.Storage["sessionStorage:WTS-BROWSER-TAB-ID"]); value != "" {
		return value
	}
	if value := strings.TrimSpace(sess.Storage["localStorage:qr-tabId"]); value != "" {
		return value
	}
	return newBrowserTabID()
}

func newBrowserTabID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "browser-tab-tossctl"
	}
	return "browser-tab-" + hex.EncodeToString(buf[:])
}

func inferAppVersion(sess *session.Session) string {
	if sess == nil {
		return ""
	}
	for name, value := range sess.Headers {
		if strings.EqualFold(name, "App-Version") {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (c *Client) ensureTradingMetadata(ctx context.Context) error {
	if strings.TrimSpace(c.browserTabID) == "" {
		c.browserTabID = newBrowserTabID()
	}
	if strings.TrimSpace(c.appVersion) != "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.tossinvest.com/account", nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	html, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	mainBundle := mainBundlePattern.FindString(string(html))
	if mainBundle == "" {
		return fmt.Errorf("could not locate tossinvest main bundle")
	}
	if strings.HasPrefix(mainBundle, "/") {
		mainBundle = "https://www.tossinvest.com" + mainBundle
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, mainBundle, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	js, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	version := appVersionPattern.FindString(string(js))
	if version == "" {
		return fmt.Errorf("could not locate tossinvest app-version")
	}
	c.appVersion = version
	return nil
}
