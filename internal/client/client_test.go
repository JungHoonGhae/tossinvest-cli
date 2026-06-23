package client

import (
	"runtime"
	"strings"
	"testing"
)

func TestBrowserUserAgentMatchesPlatform(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"darwin":  "Macintosh; Intel Mac OS X 10_15_7",
		"windows": "Windows NT 10.0; Win64; x64",
		"linux":   "X11; Linux x86_64",
		"freebsd": "X11; Linux x86_64", // unknown OS falls back to the Linux token
	}

	for goos, want := range cases {
		ua := browserUserAgent(goos)
		if !strings.Contains(ua, want) {
			t.Errorf("browserUserAgent(%q) = %q, want platform token %q", goos, ua, want)
		}
		if !strings.HasPrefix(ua, "Mozilla/5.0 (") || !strings.Contains(ua, "Chrome/") {
			t.Errorf("browserUserAgent(%q) = %q, not a plausible Chrome UA", goos, ua)
		}
	}
}

func TestDefaultBrowserUserAgentUsesHostPlatform(t *testing.T) {
	t.Parallel()

	if DefaultBrowserUserAgent != browserUserAgent(runtime.GOOS) {
		t.Fatalf("DefaultBrowserUserAgent = %q, want host-platform UA %q", DefaultBrowserUserAgent, browserUserAgent(runtime.GOOS))
	}
}
