// Package i18n provides locale-routed strings for tossctl's human-readable
// surfaces (help text, prompts, table labels). Machine-readable output
// (--json/--csv), flag names, and error strings stay English.
package i18n

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed en.json
var enBytes []byte

//go:embed ko.json
var koBytes []byte

var (
	mu       sync.RWMutex
	active   = "en"
	catalogs = map[string]map[string]string{}
)

func loadCatalog(lang string) map[string]string {
	mu.Lock()
	defer mu.Unlock()
	if c, ok := catalogs[lang]; ok {
		return c
	}
	var raw []byte
	switch lang {
	case "ko":
		raw = koBytes
	default:
		raw = enBytes
	}
	m := map[string]string{}
	_ = json.Unmarshal(raw, &m)
	catalogs[lang] = m
	return m
}

// Resolve returns "en" or "ko" from --lang flag, TOSSCTL_LANG, then LC_*/LANG.
func Resolve(args []string, getenv func(string) string) string {
	for i, a := range args {
		if a == "--lang" && i+1 < len(args) {
			return normalize(args[i+1])
		}
		if strings.HasPrefix(a, "--lang=") {
			return normalize(strings.TrimPrefix(a, "--lang="))
		}
	}
	if v := getenv("TOSSCTL_LANG"); v != "" {
		return normalize(v)
	}
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := getenv(k); v != "" {
			return normalize(v)
		}
	}
	return "en"
}

func normalize(v string) string {
	v = strings.ToLower(v)
	if strings.HasPrefix(v, "ko") {
		return "ko"
	}
	return "en"
}

// SetLang sets the active language for T().
func SetLang(lang string) {
	mu.Lock()
	active = normalize(lang)
	mu.Unlock()
}

// Lang returns the active language code.
func Lang() string {
	mu.RLock()
	defer mu.RUnlock()
	return active
}

// T returns the localized string for key; falls back to en, then the key.
// A present key is honored even if its value is empty — an empty value is
// an intentional "render nothing here," not a miss.
func T(key string) string {
	lang := Lang()
	if v, ok := loadCatalog(lang)[key]; ok {
		return v
	}
	if v, ok := loadCatalog("en")[key]; ok {
		return v
	}
	return key
}
