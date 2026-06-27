// Package onboarding contains pure decision logic for the tossctl init wizard.
// No I/O, no TUI, no network — fully unit-testable.
package onboarding

// State represents the current authentication state of the user.
type State struct {
	HasSession      bool
	HasOfficialCreds bool
}

// Method identifies an onboarding authentication method.
type Method string

const (
	MethodWeb      Method = "web"
	MethodOfficial Method = "official"
)

// NeedsOnboarding reports whether the user needs to go through onboarding.
// Returns true only when neither session nor official credentials exist.
func NeedsOnboarding(s State) bool {
	return !s.HasSession && !s.HasOfficialCreds
}

// AvailableMethods returns the supported onboarding methods in stable order.
func AvailableMethods() []Method {
	return []Method{MethodWeb, MethodOfficial}
}

// StepsFor returns the ordered list of step labels for a given method.
// Labels are user-facing Korean strings; descriptions/UI belong in the cmd layer.
// Returns nil for unknown methods.
func StepsFor(m Method) []string {
	switch m {
	case MethodOfficial:
		return []string{"키 입력", "시크릿 입력", "검증", "저장"}
	case MethodWeb:
		return []string{"브라우저 로그인"}
	default:
		return nil
	}
}
