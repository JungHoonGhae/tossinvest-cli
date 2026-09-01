package main

import (
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/routing"
)

func TestMCPOfficialBackendsRespectEffectiveRouting(t *testing.T) {
	creds := &official.Credentials{APIKey: "test-key", SecretKey: "test-secret"}
	enabled := config.File{OpenAPI: config.OpenAPI{Enabled: true}}

	for _, tc := range []struct {
		name   string
		cfg    config.File
		prefer routing.Preference
		want   bool
	}{
		{name: "auto enabled", cfg: enabled, prefer: routing.Auto, want: true},
		{name: "wts pin", cfg: enabled, prefer: routing.WTS, want: false},
		{name: "config disabled", cfg: config.File{}, prefer: routing.Auto, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			officialClient, tradingSvc := mcpOfficialBackends(tc.cfg, creds, "token.json", "lineage.json", tc.prefer)
			if got := officialClient != nil; got != tc.want {
				t.Fatalf("official client enabled = %v, want %v", got, tc.want)
			}
			if got := tradingSvc != nil; got != tc.want {
				t.Fatalf("trading service enabled = %v, want %v", got, tc.want)
			}
		})
	}
}
