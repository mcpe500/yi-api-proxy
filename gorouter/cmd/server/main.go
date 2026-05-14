package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	_ "github.com/gorouter/gorouter/internal/adapters"
	"github.com/gorouter/gorouter/internal/apikeys"
	"github.com/gorouter/gorouter/internal/audit"
	"github.com/gorouter/gorouter/internal/auth"
	"github.com/gorouter/gorouter/internal/combo"
	"github.com/gorouter/gorouter/internal/config"
	"github.com/gorouter/gorouter/internal/crypto"
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/handlers"
	"github.com/gorouter/gorouter/internal/handlers/admin"
	v1 "github.com/gorouter/gorouter/internal/handlers/v1"
	"github.com/gorouter/gorouter/internal/handlers/user"
	"github.com/gorouter/gorouter/internal/logger"
	_ "github.com/gorouter/gorouter/internal/metrics"
	ourmw "github.com/gorouter/gorouter/internal/middleware"
	"github.com/gorouter/gorouter/internal/provider"
	"github.com/gorouter/gorouter/internal/routing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const Version = "0.1.0-dev"

func findWebDir() string {
	candidates := []string{
		"web",
		filepath.Join("..", "..", "web"),
		filepath.Join("gorouter", "web"),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append([]string{filepath.Join(filepath.Dir(exe), "web")}, candidates...)
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			if abs, err := filepath.Abs(dir); err == nil {
				return abs
			}
		}
	}
	return ""
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

func serveSPA(webDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(webDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" || path == "" {
			path = "/index.html"
		}
		fp := filepath.Join(webDir, filepath.Clean(path))
		if info, err := os.Stat(fp); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("Config validation failed", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)
	log.Info("Starting gorouter", "version", Version)

	if cfg.SecretEncryptionKey == "" {
		log.Warn("GOROUTER_SECRET_ENCRYPTION_KEY is not set; provider secrets will be stored plaintext")
	} else {
		if err := crypto.Init(cfg.SecretEncryptionKey); err != nil {
			log.Error("Failed to initialize crypto", "error", err)
			os.Exit(1)
		}
		log.Info("Provider encryption initialized")
	}

	dbManager, err := db.NewDatabaseManager(cfg.NormalizedDriver(), cfg.DatabaseDSN)
	if err != nil {
		log.Error("Failed to create database manager", "error", err)
		os.Exit(1)
	}

	if err := dbManager.Connect(); err != nil {
		log.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer dbManager.Close()

	if err := dbManager.Migrate(); err != nil {
		log.Error("Failed to run migrations", "error", err)
		os.Exit(1)
	}
	log.Info("Database connected", "driver", cfg.NormalizedDriver())

	auditLogger := audit.NewAuditLogger(dbManager)

	jwtSecret := cfg.SessionSecret
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-me"
	}

	apiKeySvc := apikeys.NewApiKeyService(dbManager)

	if cfg.BootstrapAdminEmail != "" {
		if err := bootstrapAdmin(dbManager, cfg); err != nil {
			log.Error("Failed to bootstrap admin", "error", err)
			os.Exit(1)
		}
		log.Info("Admin bootstrapped", "email", cfg.BootstrapAdminEmail)
	}

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.CleanPath)

	allowCreds := !(len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*")
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"X-Request-ID", "X-RateLimit-Remaining"},
		AllowCredentials: allowCreds,
		MaxAge:           86400,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		dbStatus := "ok"
		if err := dbManager.Ping(); err != nil {
			dbStatus = "error"
		}
		ok := dbStatus == "ok"
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		w.Write([]byte(fmt.Sprintf(`{"ok":%t,"version":"%s","db":"%s"}`, ok, Version, dbStatus)))
	})

	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ready := dbManager.Ping() == nil
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		w.Write([]byte(fmt.Sprintf(`{"ready":%t}`, ready)))
	})

	r.Handle("/metrics", promhttp.Handler())

	// --- Auth routes ---
	authHandler := admin.NewAuthHandler(dbManager.Users(), jwtSecret, auditLogger)
	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)
		r.Post("/logout", authHandler.Logout)
	})

	// --- Admin routes ---
	dashboardHandler := admin.NewDashboardHandler(dbManager)
	systemHandler := admin.NewSystemHandler(cfg, Version)
	auditHandler := admin.NewAuditHandler(dbManager)
	adminApiKeyHandler := handlers.NewAdminApiKeyHandler(apiKeySvc, auditLogger)
	rateLimitHandler := admin.NewRateLimitHandler(dbManager, auditLogger)
	userHandler := admin.NewUserHandler(dbManager, auditLogger)
	providerHandler := admin.NewProviderHandler(dbManager, auditLogger)

	r.Route("/admin", func(r chi.Router) {
		r.Use(ourmw.AuditContextMiddleware())
		r.Use(ourmw.RequireAuth(jwtSecret))
		r.Use(ourmw.RequireAdmin())

		r.Route("/dashboard", func(r chi.Router) {
			r.Get("/stats", dashboardHandler.Stats)
			r.Get("/recent-activity", dashboardHandler.RecentActivity)
			r.Get("/health", dashboardHandler.Health)
		})
		r.Route("/system", func(r chi.Router) {
			r.Get("/info", systemHandler.Info)
			r.Get("/config", systemHandler.Config)
		})
		r.Route("/audit", func(r chi.Router) {
			r.Get("/logs", auditHandler.Logs)
		})
		r.Route("/models", func(r chi.Router) {
			modelHandler := handlers.NewModelHandler(dbManager, auditLogger)
			r.Get("/", modelHandler.ListModels)
			r.Post("/", modelHandler.CreateModel)
			r.Get("/{id}", modelHandler.GetModel)
			r.Put("/{id}", modelHandler.UpdateModel)
		r.Delete("/{id}", modelHandler.DeleteModel)
		})
		adminApiKeyHandler.RegisterAdminRoutes(r)
		adminApiKeyHandler.RegisterUserKeyRoutes(r)
		r.Mount("/rate-limits", rateLimitHandler.Routes())
		r.Mount("/users", userHandler.Routes())
		r.Mount("/providers", providerHandler.Routes())
	})
	userKeyHandler := handlers.NewUserKeyHandler(apiKeySvc, jwtSecret)
	userKeyHandler.RegisterRoutes(r)
	usageHandler := user.NewUsageHandler(dbManager)

	comboManager := combo.NewComboManager(dbManager)
	userCombosHandler := handlers.NewUserCombosHandler(comboManager, dbManager, jwtSecret)
	r.Route("/me", func(r chi.Router) {
		userCombosHandler.Register(r)
		r.Route("/usage", func(r chi.Router) {
			r.Use(ourmw.RequireAuth(jwtSecret))
			r.Get("/", usageHandler.History)
			r.Get("/summary", usageHandler.Summary)
		})
	})

	// --- V1 API routes ---
	router := routing.NewRouter(dbManager, routing.Strategy(cfg.RoutingStrategy))
	log.Info("Routing strategy", "strategy", cfg.RoutingStrategy)

	tokenRefresher := auth.NewTokenRefresher(dbManager)

	chatHandler := v1.NewChatHandler(dbManager, comboManager, log, router, tokenRefresher)
	responsesHandler := v1.NewResponsesHandler(dbManager, comboManager, log)
	messagesHandler := v1.NewMessagesHandler(dbManager, comboManager, log)
	modelsHandler := v1.NewModelsHandler(dbManager)
	embeddingsHandler := v1.NewEmbeddingHandler(dbManager, "")
	imagesHandler := v1.NewImagesHandler(dbManager, log)
	audioHandler := v1.NewAudioHandler(dbManager, log)
	searchHandler := v1.NewSearchHandler(dbManager, log)

	rateLimiter := ourmw.NewRateLimiter(&ourmw.RateLimitConfig{
		DB:       dbManager,
		CacheTTL: 5 * time.Minute,
		DefaultRPM: 60,
		DefaultRPD: 10000,
		DefaultTPM: 100000,
	})

	r.Route("/v1", func(r chi.Router) {
		if cfg.RequireAPIKey {
			r.Use(ourmw.RequireAPIKey(apiKeySvc))
		}
		r.Use(rateLimiter.Middleware())

		r.Post("/chat/completions", chatHandler.ServeHTTP)
		r.Post("/responses", responsesHandler.ServeHTTP)
		r.Post("/messages", messagesHandler.ServeHTTP)
		r.Get("/models", modelsHandler.ServeHTTP)
		r.Post("/embeddings", embeddingsHandler.ServeHTTP)
		r.Post("/images/generations", imagesHandler.ServeHTTP)
		r.Post("/audio/speech", audioHandler.ServeHTTP)
		r.Post("/audio/transcriptions", audioHandler.ServeHTTP)
		r.Post("/search", searchHandler.ServeHTTP)
		r.Post("/web/search", searchHandler.ServeHTTP)
	})

	// --- SPA fallback ---
	if webDir := findWebDir(); webDir != "" {
		r.Get("/*", serveSPA(webDir).ServeHTTP)
	}

	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: r,
	}

	go func() {
		log.Info("Server listening", "addr", cfg.ListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	healthMonitor := provider.NewHealthMonitor(dbManager, 30*time.Second)
	healthMonitor.Start()
	log.Info("Health monitor started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info("Shutting down", "signal", sig)

	healthMonitor.Stop()
	log.Info("Health monitor stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error("Server shutdown error", "error", err)
	}

	log.Info("Server stopped")
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func bootstrapAdmin(dbManager db.DatabaseManager, cfg *config.AppConfig) error {
	ctx := context.Background()
	users := dbManager.Users()

	existing, err := users.FindByEmail(ctx, cfg.BootstrapAdminEmail)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	hash, err := auth.HashPassword(cfg.BootstrapAdminPassword)
	if err != nil {
		return err
	}

	admin := &db.User{
		ID:           "bootstrap-admin",
		Email:        cfg.BootstrapAdminEmail,
		Name:         "Admin",
		Role:         "admin",
		Status:       "active",
		PasswordHash: hash,
	}

	return users.Create(ctx, admin)
}
