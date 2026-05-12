package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	_ "github.com/gorouter/gorouter/internal/adapters"
	"github.com/gorouter/gorouter/internal/apikeys"
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
	"github.com/gorouter/gorouter/internal/middleware"
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

	if cfg.SecretEncryptionKey != "" {
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

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"version":"` + Version + `","db":"ok"}`))
	})

	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ready":true}`))
	})

	// --- Auth routes ---
	authHandler := admin.NewAuthHandler(dbManager.Users(), jwtSecret)
	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)
		r.Post("/logout", authHandler.Logout)
	})

	// --- Admin routes ---
	dashboardHandler := admin.NewDashboardHandler(dbManager)
	systemHandler := admin.NewSystemHandler(cfg, Version)
	auditHandler := admin.NewAuditHandler(dbManager)
	adminApiKeyHandler := handlers.NewAdminApiKeyHandler(apiKeySvc)
	rateLimitHandler := admin.NewRateLimitHandler(dbManager)
	userHandler := admin.NewUserHandler(dbManager)
	providerHandler := admin.NewProviderHandler(dbManager)

	r.Route("/admin", func(r chi.Router) {
		r.Use(middleware.RequireAuth(jwtSecret))
		r.Use(middleware.RequireAdmin())

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
			modelHandler := handlers.NewModelHandler(dbManager)
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

	// --- User routes (/me/*) ---
	userKeyHandler := handlers.NewUserKeyHandler(apiKeySvc, jwtSecret)
	userKeyHandler.RegisterRoutes(r)
	usageHandler := user.NewUsageHandler(dbManager)

	comboManager := combo.NewComboManager(dbManager)
	userCombosHandler := handlers.NewUserCombosHandler(comboManager, dbManager, jwtSecret)
	r.Route("/me", func(r chi.Router) {
		userCombosHandler.Register(r)
		r.Route("/usage", func(r chi.Router) {
			r.Use(middleware.RequireAuth(jwtSecret))
			r.Get("/", usageHandler.History)
			r.Get("/summary", usageHandler.Summary)
		})
	})

	// --- V1 API routes ---
	chatHandler := &v1.ChatHandler{
		DB:     dbManager,
		Combos: comboManager,
		Logger: log,
	}
	modelsHandler := v1.NewModelsHandler(dbManager)
	embeddingsHandler := v1.NewEmbeddingHandler(dbManager, "")

	rateLimiter := middleware.NewRateLimiter(&middleware.RateLimitConfig{
		DB:       dbManager,
		CacheTTL: 5 * time.Minute,
		DefaultRPM: 60,
		DefaultRPD: 10000,
		DefaultTPM: 100000,
	})

	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.RequireAPIKey(apiKeySvc))
		r.Use(rateLimiter.Middleware())

		r.Post("/chat/completions", chatHandler.ServeHTTP)
		r.Get("/models", modelsHandler.ServeHTTP)
		r.Post("/embeddings", embeddingsHandler.ServeHTTP)
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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info("Shutting down", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error("Server shutdown error", "error", err)
	}

	log.Info("Server stopped")
}

func makeLoginHandler(dbManager db.DatabaseManager, jwtSecret string, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		user, err := dbManager.Users().FindByEmail(r.Context(), req.Email)
		if err != nil || user == nil {
			writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		if !auth.VerifyPassword(req.Password, user.PasswordHash) {
			writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		if user.Status != "active" {
			writeJSONError(w, http.StatusForbidden, "account disabled")
			return
		}

		_ = dbManager.Users().UpdateLastLogin(r.Context(), user.ID)

		token, err := auth.GenerateToken(user.ID, user.Email, user.Role, jwtSecret, 24*time.Hour)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "token generation failed")
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   86400,
		})

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"token": token,
			"user": map[string]string{
				"id":    user.ID,
				"email": user.Email,
				"name":  user.Name,
				"role":  user.Role,
			},
		})
	}
}

func makeRefreshHandler(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("auth_token")
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "missing auth token")
			return
		}

		claims, err := auth.ValidateToken(cookie.Value, jwtSecret)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		token, err := auth.GenerateToken(claims.UserID, claims.Email, claims.Role, jwtSecret, 24*time.Hour)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "token generation failed")
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   86400,
		})

		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	}
}

func makeAdminUsersRoutes(dbManager db.DatabaseManager) func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			users, err := dbManager.Users().List(r.Context(), db.UserFilter{})
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			for _, u := range users {
				u.PasswordHash = ""
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"data": users, "total": len(users)})
		})

		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Email    string `json:"email"`
				Name     string `json:"name"`
				Password string `json:"password"`
				Role     string `json:"role"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSONError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			if req.Email == "" || req.Password == "" {
				writeJSONError(w, http.StatusBadRequest, "email and password required")
				return
			}
			hash, err := auth.HashPassword(req.Password)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "password hash failed")
				return
			}
			user := &db.User{
				ID:           uuid.New().String(),
				Email:        req.Email,
				Name:         req.Name,
				Role:         req.Role,
				Status:       "active",
				PasswordHash: hash,
			}
			if user.Role == "" {
				user.Role = "user"
			}
			if err := dbManager.Users().Create(r.Context(), user); err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			user.PasswordHash = ""
			writeJSON(w, http.StatusCreated, user)
		})

		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			user, err := dbManager.Users().FindByID(r.Context(), id)
			if err != nil || user == nil {
				writeJSONError(w, http.StatusNotFound, "user not found")
				return
			}
			user.PasswordHash = ""
			writeJSON(w, http.StatusOK, user)
		})

		r.Put("/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			user, err := dbManager.Users().FindByID(r.Context(), id)
			if err != nil || user == nil {
				writeJSONError(w, http.StatusNotFound, "user not found")
				return
			}
			var req struct {
				Name   string `json:"name"`
				Role   string `json:"role"`
				Status string `json:"status"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSONError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			if req.Name != "" {
				user.Name = req.Name
			}
			if req.Role != "" {
				user.Role = req.Role
			}
			if req.Status != "" {
				user.Status = req.Status
			}
			if err := dbManager.Users().Update(r.Context(), user); err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			user.PasswordHash = ""
			writeJSON(w, http.StatusOK, user)
		})

		r.Delete("/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			if err := dbManager.Users().Delete(r.Context(), id); err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		})
	}
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
