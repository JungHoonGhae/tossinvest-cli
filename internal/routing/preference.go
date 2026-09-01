// Package routing defines the shared vocabulary used to choose the starting
// backend for hybrid requests.
package routing

// Preference selects which backend a hybrid request tries first.
type Preference string

const (
	Auto    Preference = "auto"
	WTS     Preference = "wts"
	OpenAPI Preference = "openapi"
)

// ParsePreference validates and canonicalizes a backend preference.
// "official" remains accepted as a deprecated input alias for OpenAPI.
func ParsePreference(value string) (Preference, bool) {
	switch value {
	case string(Auto):
		return Auto, true
	case string(WTS):
		return WTS, true
	case string(OpenAPI), "official":
		return OpenAPI, true
	default:
		return "", false
	}
}
