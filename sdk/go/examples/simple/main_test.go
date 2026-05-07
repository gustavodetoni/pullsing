package main

import "testing"

func TestLoadConfigRequiresAPIKey(t *testing.T) {
	t.Setenv("PULLSING_API_KEY", "")
	t.Setenv("PULLSING_ENV_API_KEY", "")
	t.Setenv("PULLSING_ADDR", "")
	t.Setenv("PULLSING_FLAG_KEY", "")

	_, _, err := loadConfig()
	if err == nil {
		t.Fatal("expected missing api key error")
	}
}

func TestLoadConfigUsesDefaultsAndAPIKey(t *testing.T) {
	t.Setenv("PULLSING_API_KEY", "psk_test")
	t.Setenv("PULLSING_ENV_API_KEY", "")
	t.Setenv("PULLSING_ADDR", "")
	t.Setenv("PULLSING_FLAG_KEY", "")

	cfg, flagKey, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.EnvAPIKey != "psk_test" {
		t.Fatalf("cfg.EnvAPIKey = %q, want psk_test", cfg.EnvAPIKey)
	}
	if cfg.Addr != defaultAddr {
		t.Fatalf("cfg.Addr = %q, want %q", cfg.Addr, defaultAddr)
	}
	if flagKey != defaultFlagKey {
		t.Fatalf("flagKey = %q, want %q", flagKey, defaultFlagKey)
	}
}

func TestLoadConfigFallsBackToEnvAPIKey(t *testing.T) {
	t.Setenv("PULLSING_API_KEY", "")
	t.Setenv("PULLSING_ENV_API_KEY", "psk_from_env_api_key")
	t.Setenv("PULLSING_ADDR", "127.0.0.1:6000")
	t.Setenv("PULLSING_FLAG_KEY", "new-button")

	cfg, flagKey, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.EnvAPIKey != "psk_from_env_api_key" {
		t.Fatalf("cfg.EnvAPIKey = %q, want psk_from_env_api_key", cfg.EnvAPIKey)
	}
	if cfg.Addr != "127.0.0.1:6000" {
		t.Fatalf("cfg.Addr = %q, want 127.0.0.1:6000", cfg.Addr)
	}
	if flagKey != "new-button" {
		t.Fatalf("flagKey = %q, want new-button", flagKey)
	}
}
