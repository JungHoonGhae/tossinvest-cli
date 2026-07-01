package i18n

import (
	"strings"
	"testing"
	"unicode"
)

func hasHangul(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Hangul) {
			return true
		}
	}
	return false
}

func TestCatalogParity(t *testing.T) {
	en := loadCatalog("en")
	ko := loadCatalog("ko")
	for k := range en {
		if _, ok := ko[k]; !ok {
			t.Errorf("ko catalog missing key %q", k)
		}
	}
	for k := range ko {
		if _, ok := en[k]; !ok {
			t.Errorf("orphan ko key not in en: %q", k)
		}
	}
}

func TestEnglishShortKeysAreHangulFree(t *testing.T) {
	for k, v := range loadCatalog("en") {
		if strings.HasSuffix(k, ".short") {
			if hasHangul(v) {
				t.Errorf("en %q contains Hangul: %q", k, v)
			}
			if strings.HasSuffix(v, ".") {
				t.Errorf("en %q ends with period: %q", k, v)
			}
		}
	}
}

func TestResolvePrecedence(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	// --lang wins
	if got := Resolve([]string{"--lang", "ko"}, env(map[string]string{"LANG": "en_US"})); got != "ko" {
		t.Errorf("--lang ko => %q, want ko", got)
	}
	// TOSSCTL_LANG next
	if got := Resolve(nil, env(map[string]string{"TOSSCTL_LANG": "ko"})); got != "ko" {
		t.Errorf("TOSSCTL_LANG => %q, want ko", got)
	}
	// LANG prefix
	if got := Resolve(nil, env(map[string]string{"LANG": "ko_KR.UTF-8"})); got != "ko" {
		t.Errorf("LANG ko_KR => %q, want ko", got)
	}
	// default en
	if got := Resolve(nil, env(map[string]string{})); got != "en" {
		t.Errorf("empty => %q, want en", got)
	}
}

func TestTFallback(t *testing.T) {
	SetLang("ko")
	if T("_meta.lang") != "ko" {
		t.Errorf("T(_meta.lang) under ko = %q, want ko", T("_meta.lang"))
	}
	if T("nonexistent.key") != "nonexistent.key" {
		t.Errorf("missing key should return key itself")
	}
}
