package onboarding_test

import (
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/onboarding"
)

func TestNeedsOnboarding(t *testing.T) {
	cases := []struct {
		name string
		s    onboarding.State
		want bool
	}{
		{"둘 다 없음", onboarding.State{HasSession: false, HasOfficialCreds: false}, true},
		{"세션만 있음", onboarding.State{HasSession: true, HasOfficialCreds: false}, false},
		{"공식키만 있음", onboarding.State{HasSession: false, HasOfficialCreds: true}, false},
		{"둘 다 있음", onboarding.State{HasSession: true, HasOfficialCreds: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := onboarding.NeedsOnboarding(tc.s)
			if got != tc.want {
				t.Errorf("NeedsOnboarding(%+v) = %v, want %v", tc.s, got, tc.want)
			}
		})
	}
}

func TestAvailableMethods(t *testing.T) {
	methods := onboarding.AvailableMethods()
	want := []onboarding.Method{onboarding.MethodWeb, onboarding.MethodOfficial}
	if len(methods) != len(want) {
		t.Fatalf("AvailableMethods() len = %d, want %d", len(methods), len(want))
	}
	for i, m := range methods {
		if m != want[i] {
			t.Errorf("AvailableMethods()[%d] = %q, want %q", i, m, want[i])
		}
	}
}

func TestStepsFor(t *testing.T) {
	cases := []struct {
		method onboarding.Method
		want   []string
	}{
		{
			onboarding.MethodOfficial,
			[]string{"키 입력", "시크릿 입력", "검증", "저장"},
		},
		{
			onboarding.MethodWeb,
			[]string{"브라우저 로그인"},
		},
		{
			onboarding.Method("unknown"),
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(string(tc.method), func(t *testing.T) {
			got := onboarding.StepsFor(tc.method)
			if len(got) != len(tc.want) {
				t.Fatalf("StepsFor(%q) len = %d, want %d; got %v", tc.method, len(got), len(tc.want), got)
			}
			for i, s := range got {
				if s != tc.want[i] {
					t.Errorf("StepsFor(%q)[%d] = %q, want %q", tc.method, i, s, tc.want[i])
				}
			}
		})
	}
}
