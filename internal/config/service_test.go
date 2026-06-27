package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStatusFallsBackToDefaultWhenConfigIsMissing(t *testing.T) {
	service := NewService(filepath.Join(t.TempDir(), "config.json"))

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.Exists {
		t.Fatal("expected config to be absent")
	}
	if status.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schema version %d, got %d", SchemaVersion, status.SchemaVersion)
	}
	if status.Trading.Place {
		t.Fatal("expected place to be disabled by default")
	}
	if status.Trading.AllowLiveOrderActions {
		t.Fatal("expected live order actions to be disabled by default")
	}
}

func TestInitCreatesDefaultConfig(t *testing.T) {
	service := NewService(filepath.Join(t.TempDir(), "config.json"))

	result, err := service.Init(context.Background())
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if !result.Created {
		t.Fatal("expected config file to be created")
	}
	if !result.Status.Exists {
		t.Fatal("expected config file to exist after init")
	}
	if result.Status.Schema != DefaultSchemaURL {
		t.Fatalf("expected schema url %q, got %q", DefaultSchemaURL, result.Status.Schema)
	}
	if result.Status.Trading.AllowLiveOrderActions {
		t.Fatal("expected live order actions to be disabled by default")
	}
}

func TestLoadTranslatesLegacyAllowDangerousExecute(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{
  "$schema": "https://raw.githubusercontent.com/JungHoonGhae/tossinvest-cli/main/schemas/config.schema.json",
  "schema_version": 1,
  "trading": {
    "place": true,
    "cancel": false,
    "amend": false,
    "allow_dangerous_execute": true
  }
}`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(configPath)

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.SchemaVersion != SchemaVersion {
		t.Fatalf("expected effective schema version %d, got %d", SchemaVersion, status.SchemaVersion)
	}
	if status.SourceSchemaVersion != 1 {
		t.Fatalf("expected source schema version 1, got %d", status.SourceSchemaVersion)
	}
	if !status.Trading.AllowLiveOrderActions {
		t.Fatal("expected legacy allow_dangerous_execute to translate into allow_live_order_actions")
	}
	if len(status.LegacyFields) != 1 || status.LegacyFields[0] != "trading.allow_dangerous_execute" {
		t.Fatalf("unexpected legacy fields: %#v", status.LegacyFields)
	}
}

func TestLoadDefaultsSellToFalseWhenFieldMissing(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{
  "schema_version": 2,
  "trading": {
    "place": true,
    "cancel": false,
    "amend": false,
    "allow_live_order_actions": true,
    "dangerous_automation": {
      "accept_fx_consent": false
    }
  }
}`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(configPath)
	cfg, err := service.Load(context.Background())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Trading.Sell {
		t.Fatal("expected Sell to default to false when field is missing from config")
	}
	if !cfg.Trading.Place {
		t.Fatal("expected Place to be true")
	}
}

func TestLoadParsesSellTrue(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{
  "schema_version": 2,
  "trading": {
    "place": true,
    "sell": true,
    "cancel": false,
    "amend": false,
    "allow_live_order_actions": true,
    "dangerous_automation": {
      "accept_fx_consent": false
    }
  }
}`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(configPath)
	cfg, err := service.Load(context.Background())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Trading.Sell {
		t.Fatal("expected Sell to be true")
	}
}

func TestEnabledActionsIncludesSell(t *testing.T) {
	trading := Trading{
		Place: true,
		Sell:  true,
	}
	enabled := trading.EnabledActions()
	found := false
	for _, action := range enabled {
		if action == "sell" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected EnabledActions to include 'sell', got %v", enabled)
	}
}

func TestEnabledActionsExcludesSellWhenFalse(t *testing.T) {
	trading := Trading{
		Place: true,
		Sell:  false,
	}
	enabled := trading.EnabledActions()
	for _, action := range enabled {
		if action == "sell" {
			t.Fatalf("expected EnabledActions to exclude 'sell' when false, got %v", enabled)
		}
	}
}

func TestLoadFlagsKRAsLegacyField(t *testing.T) {
	// trading.kr was removed in v0.5.2 — a config that still carries it must
	// load fine (the value is ignored) and surface "trading.kr" as a legacy
	// field so the doctor / startup warning can flag it.
	configPath := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{
  "schema_version": 2,
  "trading": {
    "place": true,
    "sell": true,
    "kr": true,
    "cancel": false,
    "amend": false,
    "allow_live_order_actions": true,
    "dangerous_automation": {
      "accept_fx_consent": false
    }
  }
}`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(configPath)
	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	found := false
	for _, f := range status.LegacyFields {
		if f == "trading.kr" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected trading.kr in LegacyFields, got %v", status.LegacyFields)
	}
}

func TestUpdateCheckDefaultsToEnabled(t *testing.T) {
	// 미존재 config: default file 가 UpdateCheck.Enabled = true 여야 한다.
	missing := NewService(filepath.Join(t.TempDir(), "config.json"))
	status, err := missing.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if !status.UpdateCheck.Enabled {
		t.Fatal("expected update_check to default to enabled when config is missing")
	}

	// update_check 필드 미지정 config: 호환성을 위해 enabled 로 해석되어야 한다.
	legacyPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(legacyPath, []byte(`{"schema_version":2,"trading":{}}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := NewService(legacyPath).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.UpdateCheck.Enabled {
		t.Fatal("expected update_check to default to enabled when field is omitted")
	}
}

func TestUpdateCheckExplicitFalseIsRespected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":2,"trading":{},"update_check":{"enabled":false}}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := NewService(path).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.UpdateCheck.Enabled {
		t.Fatal("expected explicit update_check.enabled=false to be respected")
	}
}

func TestInitCreatesDangerousAutomationDefaults(t *testing.T) {
	service := NewService(filepath.Join(t.TempDir(), "config.json"))

	result, err := service.Init(context.Background())
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if result.Status.Trading.DangerousAutomation.AcceptFXConsent {
		t.Fatal("expected accept_fx_consent to be disabled by default")
	}
}

func TestLoadDefaultsOpenAPI(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(filepath.Join(dir, "config.json"))
	cfg, err := svc.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.OpenAPI.Enabled || cfg.OpenAPI.Prefer != "auto" || !cfg.OpenAPI.Fallback {
		t.Fatalf("unexpected openapi defaults: %+v", cfg.OpenAPI)
	}
}

func TestLoadOpenAPIFromExistingConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{
  "schema_version": 3,
  "trading": {},
  "openapi": {
    "enabled": true,
    "prefer": "openapi",
    "fallback": false
  }
}`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := NewService(configPath).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.OpenAPI.Enabled {
		t.Fatal("expected openapi.enabled to be true")
	}
	if cfg.OpenAPI.Prefer != "openapi" {
		t.Fatalf("expected prefer=openapi, got %q", cfg.OpenAPI.Prefer)
	}
	if cfg.OpenAPI.Fallback {
		t.Fatal("expected openapi.fallback to be false")
	}
}

func TestLoadOpenAPINormalizesInvalidPrefer(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{
  "schema_version": 3,
  "trading": {},
  "openapi": {
    "enabled": true,
    "prefer": "bogus",
    "fallback": true
  }
}`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := NewService(configPath).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OpenAPI.Prefer != "auto" {
		t.Fatalf("expected invalid prefer to normalize to 'auto', got %q", cfg.OpenAPI.Prefer)
	}
}

func TestLoadOpenAPIMissingBlockDefaultsApplied(t *testing.T) {
	// Old config without openapi block should get defaults.
	configPath := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"schema_version":2,"trading":{}}`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := NewService(configPath).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.OpenAPI.Enabled || cfg.OpenAPI.Prefer != "auto" || !cfg.OpenAPI.Fallback {
		t.Fatalf("expected defaults when openapi block missing, got: %+v", cfg.OpenAPI)
	}
}

func TestDefaultPathsIncludesOpenAPIFiles(t *testing.T) {
	paths, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths: %v", err)
	}
	if paths.CredentialsFile == "" {
		t.Fatal("expected CredentialsFile to be set")
	}
	if paths.TokenFile == "" {
		t.Fatal("expected TokenFile to be set")
	}
}
