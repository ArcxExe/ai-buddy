package main

import (
	"aibuddy/backend/internal/ai"
	httpapi "aibuddy/backend/internal/http"
	"aibuddy/backend/internal/mcp"
	"aibuddy/backend/internal/rag"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func main() {
	port := getEnv("BACKEND_PORT", "8080")
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	baseURL := getEnv("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1")
	model := getEnv("OPENROUTER_MODEL", "google/gemini-2.5-flash")

	log.Printf("[AI-buddy] Starting backend server (Go 1.27.3)...")
	log.Printf("[AI-buddy] LLM Provider: OpenRouter (model: %s)", model)
	if apiKey == "" {
		log.Printf("[AI-buddy] Notice: OPENROUTER_API_KEY is not set. Smart SDD Fallback Engine is active (100%% offline hackathon proof).")
	} else {
		log.Printf("[AI-buddy] OpenRouter API key detected. Live LLM calls enabled.")
	}

	// 1. Initialize State and Services
	store := mcp.NewStore()
	executor := mcp.NewExecutor(store)

	kb := rag.NewKnowledgeBase()
	seedPaths := []string{
		"data/knowledge/avito_playbook_seed.json",
		"../data/knowledge/avito_playbook_seed.json",
		"../../data/knowledge/avito_playbook_seed.json",
	}
	loaded := false
	for _, p := range seedPaths {
		if err := kb.LoadFromJSON(p); err == nil {
			log.Printf("[AI-buddy] Loaded knowledge base from %s", p)
			loaded = true
			break
		}
	}
	if !loaded {
		log.Printf("[AI-buddy] Warning: Could not find seed JSON file, starting with default empty KB.")
	}

	agentCfg := ai.Config{
		Provider:       "openrouter",
		APIKey:         apiKey,
		BaseURL:        baseURL,
		Model:          model,
		Timeout:        15 * time.Second,
		EnableFallback: true,
	}
	agent := ai.NewAgentOrchestrator(agentCfg, kb, executor)

	apiServer := httpapi.NewServer(store, executor, kb, agent)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      apiServer.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
	}

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[AI-buddy] Server listening on http://0.0.0.0:%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[AI-buddy] HTTP server failed: %v", err)
		}
	}()

	<-stop
	log.Printf("[AI-buddy] Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Printf("[AI-buddy] Server stopped.")
}
