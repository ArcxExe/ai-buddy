package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg := LoadWithLookup(func(string) (string, bool) {
		return "", false
	})

	if cfg.AppEnv != "development" {
		t.Fatalf("expected development env, got %s", cfg.AppEnv)
	}
	if cfg.BackendPort != "8080" || cfg.ListenAddr() != ":8080" {
		t.Fatalf("unexpected port config: %s / %s", cfg.BackendPort, cfg.ListenAddr())
	}
	if cfg.LLMTimeout != 25*time.Second {
		t.Fatalf("expected 25s timeout, got %v", cfg.LLMTimeout)
	}
	if cfg.LLMMaxTokens != 1024 {
		t.Fatalf("expected default LLMMaxTokens 1024, got %d", cfg.LLMMaxTokens)
	}
	if cfg.StorageMode != "memory" {
		t.Fatalf("expected memory storage mode, got %s", cfg.StorageMode)
	}
	if cfg.OpenRouterEnabled() {
		t.Fatalf("expected OpenRouter disabled when API key empty")
	}

	flags := cfg.FeatureFlags.ToMap()
	for _, k := range []string{"mcp_enabled", "mcp_actions", "rbac_strict", "clickstream_analytics", "proactive_cron_hints"} {
		if !flags[k] {
			t.Fatalf("expected default flag %s to be true", k)
		}
	}
}

func TestLoadCustomValuesAndInvalidFallbacks(t *testing.T) {
	env := map[string]string{
		"APP_ENV":                         "production",
		"BACKEND_PORT":                    ":9090",
		"CORS_ALLOWED_ORIGINS":            "http://localhost:4173, https://ai-buddy.local",
		"LLM_PROVIDER":                    "OpenRouter",
		"OPENROUTER_API_KEY":              "sk-or-test-123",
		"OPENROUTER_BASE_URL":             "https://openrouter.ai/api/v1/",
		"OPENROUTER_MODEL":                "anthropic/claude-3.5-sonnet",
		"LLM_TIMEOUT_MS":                  "invalid",
		"LLM_MAX_TOKENS":                  "2048",
		"STORAGE_MODE":                    "POSTGRES",
		"POSTGRES_DSN":                    "postgres://aibuddy:secret@postgres:5432/aibuddy?sslmode=disable",
		"REDIS_ADDR":                      "redis:6379",
		"REDIS_PASSWORD":                  "pass",
		"FF_ENABLE_MCP_ACTIONS":           "false",
		"FF_ENABLE_RBAC_STRICT":           "not-a-bool",
		"FF_ENABLE_CLICKSTREAM_ANALYTICS": "false",
		"FF_ENABLE_PROACTIVE_CRON_HINTS":  "false",
	}

	cfg := LoadWithLookup(func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	})

	if cfg.ListenAddr() != ":9090" {
		t.Fatalf("expected :9090, got %s", cfg.ListenAddr())
	}
	if len(cfg.CORSAllowedOrigins) != 2 || cfg.CORSAllowedOrigins[1] != "https://ai-buddy.local" {
		t.Fatalf("unexpected CORS origins: %#v", cfg.CORSAllowedOrigins)
	}
	if !cfg.OpenRouterEnabled() {
		t.Fatalf("expected OpenRouter enabled")
	}
	if cfg.OpenRouterBaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("expected trailing slash trimmed, got %s", cfg.OpenRouterBaseURL)
	}
	if cfg.LLMTimeout != 25*time.Second {
		t.Fatalf("expected fallback 25s on invalid timeout, got %v", cfg.LLMTimeout)
	}
	if cfg.LLMMaxTokens != 2048 {
		t.Fatalf("expected LLMMaxTokens 2048, got %d", cfg.LLMMaxTokens)
	}
	if cfg.StorageMode != "postgres" {
		t.Fatalf("expected lowercase postgres mode, got %s", cfg.StorageMode)
	}
	if cfg.FeatureFlags.MCPActions || !cfg.FeatureFlags.RBACStrict || cfg.FeatureFlags.ClickstreamAnalytics || cfg.FeatureFlags.ProactiveCronHints {
		t.Fatalf("unexpected feature flags: %+v", cfg.FeatureFlags)
	}
}

func TestLoadEdgeCases(t *testing.T) {
	env := map[string]string{
		"BACKEND_PORT":         "   ",
		"LLM_TIMEOUT_MS":       "-500",
		"LLM_MAX_TOKENS":       "0",
		"STORAGE_MODE":         "unknown-db",
		"CORS_ALLOWED_ORIGINS": " , , ",
	}
	cfg := LoadWithLookup(func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	})

	if cfg.StorageMode != "memory" {
		t.Fatalf("expected fallback to memory, got %s", cfg.StorageMode)
	}
	if cfg.LLMTimeout != 25*time.Second {
		t.Fatalf("expected fallback timeout on negative value, got %v", cfg.LLMTimeout)
	}
	if cfg.LLMMaxTokens != 1024 {
		t.Fatalf("expected fallback LLMMaxTokens 1024 on non-positive value, got %d", cfg.LLMMaxTokens)
	}
	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("expected default CORS origins on empty CSV, got %#v", cfg.CORSAllowedOrigins)
	}
	if (Config{}).ListenAddr() != ":8080" {
		t.Fatalf("expected empty Config ListenAddr to default to :8080")
	}
	_ = Load()
}

func TestStorageModeAutoSelection(t *testing.T) {
	const dsn = "postgres://aibuddy:secret@postgres:5432/aibuddy?sslmode=disable"

	cases := []struct {
		name     string
		env      map[string]string
		expected string
	}{
		{
			name:     "unset STORAGE_MODE with non-empty POSTGRES_DSN defaults to postgres",
			env:      map[string]string{"POSTGRES_DSN": dsn},
			expected: "postgres",
		},
		{
			name:     "empty STORAGE_MODE with non-empty POSTGRES_DSN defaults to postgres",
			env:      map[string]string{"STORAGE_MODE": "  ", "POSTGRES_DSN": dsn},
			expected: "postgres",
		},
		{
			name:     "explicit STORAGE_MODE=memory with non-empty POSTGRES_DSN honors memory",
			env:      map[string]string{"STORAGE_MODE": "memory", "POSTGRES_DSN": dsn},
			expected: "memory",
		},
		{
			name:     "unset STORAGE_MODE with empty POSTGRES_DSN defaults to memory",
			env:      map[string]string{"POSTGRES_DSN": ""},
			expected: "memory",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := LoadWithLookup(func(key string) (string, bool) {
				v, ok := tc.env[key]
				return v, ok
			})
			if cfg.StorageMode != tc.expected {
				t.Fatalf("expected StorageMode %q, got %q", tc.expected, cfg.StorageMode)
			}
		})
	}
}
