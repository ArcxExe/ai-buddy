package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"aibuddy/backend/internal/ai"
	"aibuddy/backend/internal/config"
	"aibuddy/backend/internal/domain"
	httpapi "aibuddy/backend/internal/http"
	"aibuddy/backend/internal/mcp"
	"aibuddy/backend/internal/rag"
	"aibuddy/backend/internal/storage"
	"aibuddy/backend/internal/telemetry"
)

func main() {
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Load()

	repo := storage.NewRepository(rootCtx, cfg)
	defer func() {
		if err := repo.Close(); err != nil {
			log.Printf("WARN: repository close error: %v", err)
		}
	}()

	tracker := telemetry.NewTracker(cfg.FeatureFlags.ClickstreamAnalytics)
	tracker.SetSink(func(ev telemetry.ClickstreamEvent) {
		if repo.Mode() == "memory" {
			_ = repo.AppendClickstream(rootCtx, storage.ClickstreamRecord{
				ID:        ev.ID,
				EventType: ev.EventType,
				ActorRole: ev.ActorRole,
				ActorID:   ev.ActorID,
				Route:     ev.Route,
				LatencyMs: ev.LatencyMs,
				Metadata:  ev.Metadata,
				Timestamp: ev.Timestamp,
			})
			return
		}
		go func(event telemetry.ClickstreamEvent) {
			ctx, stop := context.WithTimeout(rootCtx, time.Second)
			defer stop()
			_ = repo.AppendClickstream(ctx, storage.ClickstreamRecord{
				ID:        event.ID,
				EventType: event.EventType,
				ActorRole: event.ActorRole,
				ActorID:   event.ActorID,
				Route:     event.Route,
				LatencyMs: event.LatencyMs,
				Metadata:  event.Metadata,
				Timestamp: event.Timestamp,
			})
		}(ev)
	})

	store := mcp.NewStoreWithRepo(rootCtx, repo)

	kb := rag.NewKnowledgeBase()
	if seedPath := resolveKnowledgePath(cfg.KnowledgePath); seedPath != "" {
		if err := kb.LoadFromJSON(seedPath); err != nil {
			log.Printf("WARN: could not load knowledge seed %s: %v", seedPath, err)
		} else {
			articles, _ := kb.ListAllVisible(domain.RoleManager)
			store.SetArticles(articles)
			log.Printf("INFO: Loaded %d knowledge articles from %s", len(articles), seedPath)
		}
	} else {
		log.Printf("WARN: knowledge seed file not found, starting with empty KB")
	}

	exec := mcp.NewExecutorWithDeps(store, kb, tracker)
	aiCfg := ai.Config{
		Provider:       cfg.LLMProvider,
		APIKey:         cfg.OpenRouterAPIKey,
		BaseURL:        cfg.OpenRouterBaseURL,
		Model:          cfg.OpenRouterModel,
		Timeout:        cfg.LLMTimeout,
		MaxTokens:      cfg.LLMMaxTokens,
		EnableFallback: true,
	}
	agent := ai.NewAgentOrchestratorWithDeps(aiCfg, kb, exec, exec, nil, tracker, cfg.FeatureFlags.RBACStrict)

	srv := httpapi.NewServerWithConfig(cfg, store, exec, kb, agent, tracker)
	srv.StartProactiveHintWorker(rootCtx, 10*time.Minute)

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr(),
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("AI-buddy Backend listening on %s (storage=%s, openrouter_enabled=%t, model=%s)",
			cfg.ListenAddr(), repo.Mode(), cfg.OpenRouterEnabled(), cfg.OpenRouterModel)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down AI-buddy Backend cleanly...")
	cancel()
	ctx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
}

func resolveKnowledgePath(override string) string {
	candidates := []string{
		override,
		filepath.Join("data", "knowledge", "avito_playbook_seed.json"),
		filepath.Join("..", "data", "knowledge", "avito_playbook_seed.json"),
		filepath.Join("..", "..", "data", "knowledge", "avito_playbook_seed.json"),
		filepath.Join("/app", "data", "knowledge", "avito_playbook_seed.json"),
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}
