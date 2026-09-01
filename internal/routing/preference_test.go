package routing

import "testing"

func TestParsePreference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want Preference
		ok   bool
	}{
		{name: "auto", in: "auto", want: Auto, ok: true},
		{name: "wts", in: "wts", want: WTS, ok: true},
		{name: "openapi", in: "openapi", want: OpenAPI, ok: true},
		{name: "deprecated official alias", in: "official", want: OpenAPI, ok: true},
		{name: "unknown", in: "invalid"},
		{name: "empty", in: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ParsePreference(tt.in)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("ParsePreference(%q) = (%q, %t), want (%q, %t)", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}
