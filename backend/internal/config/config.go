package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// FeatureFlags holds runtime toggles mapped to FF_ENABLE_* environment variables.
type FeatureFlags struct {
	MCPActions           bool
	RBACStrict           bool
	ClickstreamAnalytics bool
	ProactiveCronHints   bool
}

// ToMap serializes feature flags into the API contract format consumed by the frontend.
func (f FeatureFlags) ToMap() map[string]bool {
	return map[string]bool{
		"mcp_enabled":           f.MCPActions,
		"mcp_actions":           f.MCPActions,
		"rbac_strict":           f.RBACStrict,
		"clickstream_analytics": f.ClickstreamAnalytics,
		"proactive_cron_hints":  f.ProactiveCronHints,
	}
}

// Config centralizes all backend environment configuration matching .env.example.
type Config struct {
	AppEnv             string
	BackendPort        string
	CORSAllowedOrigins []string

	LLMProvider       string
	OpenRouterAPIKey  string
	OpenRouterBaseURL string
	OpenRouterModel   string
	LLMTimeout        time.Duration

	StorageMode   string
	PostgresDSN   string
	RedisAddr     string
	RedisPassword string
	KnowledgePath string

	FeatureFlags FeatureFlags
}

// LookupFunc mirrors os.LookupEnv for deterministic dependency-injected testing.
type LookupFunc func(key string) (string, bool)

// Load reads configuration from process environment variables.
func Load() Config {
	return LoadWithLookup(os.LookupEnv)
}

// LoadWithLookup loads configuration using the supplied lookup function and applies safe defaults.
func LoadWithLookup(lookup LookupFunc) Config {
	if lookup == nil {
		lookup = os.LookupEnv
	}

	timeoutMs := parseInt(lookup, "LLM_TIMEOUT_MS", 25000)
	if timeoutMs <= 0 {
		timeoutMs = 25000
	}

	storageMode := strings.ToLower(strings.TrimSpace(parseString(lookup, "STORAGE_MODE", "memory")))
	if storageMode != "memory" && storageMode != "postgres" {
		storageMode = "memory"
	}

	return Config{
		AppEnv:             parseString(lookup, "APP_ENV", "development"),
		BackendPort:        parseString(lookup, "BACKEND_PORT", "8080"),
		CORSAllowedOrigins: parseCSV(lookup, "CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173", "http://localhost:3000"}),

		LLMProvider:       strings.ToLower(parseString(lookup, "LLM_PROVIDER", "openrouter")),
		OpenRouterAPIKey:  strings.TrimSpace(parseString(lookup, "OPENROUTER_API_KEY", "")),
		OpenRouterBaseURL: strings.TrimRight(parseString(lookup, "OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"), "/"),
		OpenRouterModel:   parseString(lookup, "OPENROUTER_MODEL", "google/gemini-2.5-flash"),
		LLMTimeout:        time.Duration(timeoutMs) * time.Millisecond,

		StorageMode:   storageMode,
		PostgresDSN:   strings.TrimSpace(parseString(lookup, "POSTGRES_DSN", "")),
		RedisAddr:     strings.TrimSpace(parseString(lookup, "REDIS_ADDR", "")),
		RedisPassword: parseString(lookup, "REDIS_PASSWORD", ""),
		KnowledgePath: strings.TrimSpace(parseString(lookup, "KNOWLEDGE_PATH", "")),

		FeatureFlags: FeatureFlags{
			MCPActions:           parseBool(lookup, "FF_ENABLE_MCP_ACTIONS", true),
			RBACStrict:           parseBool(lookup, "FF_ENABLE_RBAC_STRICT", true),
			ClickstreamAnalytics: parseBool(lookup, "FF_ENABLE_CLICKSTREAM_ANALYTICS", true),
			ProactiveCronHints:   parseBool(lookup, "FF_ENABLE_PROACTIVE_CRON_HINTS", true),
		},
	}
}

// OpenRouterEnabled reports whether external OpenRouter inference should be attempted.
func (c Config) OpenRouterEnabled() bool {
	return c.LLMProvider == "openrouter" && c.OpenRouterAPIKey != ""
}

// ListenAddr returns the HTTP bind address derived from BackendPort.
func (c Config) ListenAddr() string {
	port := strings.TrimSpace(c.BackendPort)
	if port == "" {
		return ":8080"
	}
	if strings.HasPrefix(port, ":") {
		return port
	}
	return ":" + port
}

func parseString(lookup LookupFunc, key, fallback string) string {
	val, ok := lookup(key)
	if !ok || strings.TrimSpace(val) == "" {
		return fallback
	}
	return strings.TrimSpace(val)
}

func parseInt(lookup LookupFunc, key string, fallback int) int {
	val, ok := lookup(key)
	if !ok || strings.TrimSpace(val) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		return fallback
	}
	return parsed
}

func parseBool(lookup LookupFunc, key string, fallback bool) bool {
	val, ok := lookup(key)
	if !ok || strings.TrimSpace(val) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(val))
	if err != nil {
		return fallback
	}
	return parsed
}

func parseCSV(lookup LookupFunc, key string, fallback []string) []string {
	val, ok := lookup(key)
	if !ok || strings.TrimSpace(val) == "" {
		out := make([]string, len(fallback))
		copy(out, fallback)
		return out
	}
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		out = make([]string, len(fallback))
		copy(out, fallback)
	}
	return out
}
