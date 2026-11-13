package storage

import (
	"context"
	"log"
	"time"

	"aibuddy/backend/internal/config"
)

// NewRepository constructs the runtime Repository based on STORAGE_MODE, POSTGRES_DSN,
// and REDIS_ADDR, gracefully degrading to in-memory storage if external stores are offline.
func NewRepository(ctx context.Context, cfg config.Config) Repository {
	var primary Repository = NewMemoryRepository()

	if cfg.StorageMode == "postgres" && cfg.PostgresDSN != "" {
		pg, err := NewPostgresRepository(cfg.PostgresDSN, 2*time.Second)
		if err != nil {
			log.Printf("WARN: PostgreSQL config invalid (%v); falling back to memory store", err)
		} else {
			var initErr error
			for attempt := 0; attempt < 3; attempt++ {
				probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				initErr = pg.InitSchema(probeCtx)
				cancel()
				if initErr == nil {
					primary = pg
					break
				}
				select {
				case <-ctx.Done():
					attempt = 3
				case <-time.After(300 * time.Millisecond):
				}
			}
			if initErr != nil {
				log.Printf("WARN: PostgreSQL unavailable (%v); using safe in-memory fallback", initErr)
			}
		}
	}

	if cfg.RedisAddr != "" {
		rc := NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, 800*time.Millisecond)
		probeCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
		err := rc.Ping(probeCtx)
		cancel()
		if err != nil {
			log.Printf("WARN: Redis cache unavailable at %s (%v); running without Redis layer", cfg.RedisAddr, err)
		} else {
			return NewCachedRepository(primary, rc)
		}
	}

	return primary
}
