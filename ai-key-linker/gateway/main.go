package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-gateway/config"
	"ai-gateway/db"
	"ai-gateway/handler"
	"ai-gateway/limiter"
	"ai-gateway/models"
	"ai-gateway/proxy"
)

func main() {
	log.Println("[INFO] Starting AI Gateway / Smart Proxy...")

	cfg := config.LoadConfig()

	// Initialize Database
	database, err := db.InitDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize database: %v", err)
	}
	log.Printf("[INFO] SQLite database initialized at %s with WAL mode", cfg.DBPath)

	// Seed Upstream Providers (CodeCraft accounts) if empty
	seedProviders(database)

	// Initialize Limiters & Engines
	rpmLimiter := limiter.NewRPMLimiter()
	concLimiter := limiter.NewConcurrencyLimiter()
	upstreamPool := proxy.NewUpstreamPool(database)
	relayEngine := proxy.NewRelayEngine(database, upstreamPool, cfg)
	apiHandler := handler.NewAPIHandler(cfg, database, rpmLimiter, concLimiter, relayEngine)

	mux := http.NewServeMux()
	apiHandler.RegisterRoutes(mux)

	// Wrap with CORS and middleware
	handlerWithMiddleware := apiHandler.WrapCORS(mux)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handlerWithMiddleware,
		ReadTimeout:  cfg.RequestTimeout,
		WriteTimeout: cfg.RequestTimeout,
		IdleTimeout:  cfg.StreamIdleTimeout,
	}

	// Graceful shutdown handling
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[INFO] AI Gateway listening on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Server error: %v", err)
		}
	}()

	<-stopChan
	log.Println("[INFO] Shutting down AI Gateway...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[ERROR] Shutdown error: %v", err)
	}

	log.Println("[INFO] AI Gateway stopped cleanly.")
}

func seedProviders(database *db.DB) {
	providers, err := database.GetAllProviders()
	if err == nil && len(providers) == 0 {
		codeCraftKeys := []string{
			"cc_U2sE4bhkp2nS925z9z4bUxvEo9C7XiDGy15gNJSDAlKAWaDQ",
			"cc_VFT2X0x38G4oAwytuuvyvXsAwpoHfG5mnk8GBxvSTpg6Kq5m",
			"cc_BcoIOlNN0towuKkBW7HO3kGFAEdxxmEq4OpwNVjVUBIEEf9K",
			"cc_VAlHpouQPhYZXHfGCMW75Rp1XHimXps4bjQ99uWAIdz5L09A",
		}

		for i, key := range codeCraftKeys {
			p := &models.Provider{
				Name:            "codecraft-account-" + string(rune('1'+i)),
				BaseURL:         "https://codecraftapi.com/v1",
				APIKey:          key,
				SupportedModels: "claude-opus-5.5,claude-sonnet-3.5,*",
				Priority:        i + 1, // 1 to 4 priority
				Weight:          10,
				Status:          models.StatusActive,
			}
			_ = database.InsertProvider(p)
		}
		log.Println("[INFO] Seeded 4 CodeCraft upstream provider keys with priority failover")
	}
}
