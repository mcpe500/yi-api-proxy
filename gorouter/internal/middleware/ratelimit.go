package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorouter/gorouter/internal/db"
)

type RateLimitConfig struct {
	DB             db.DatabaseManager
	CacheTTL       time.Duration
	DefaultRPM     int
	DefaultRPD     int
	DefaultTPM     int
}

type RateLimiter struct {
	cfg    *RateLimitConfig
	cache  map[string]*cachedLimit
	mu     sync.RWMutex
	window map[string]*slidingWindow
	wmu    sync.RWMutex
}

type cachedLimit struct {
	limit      *db.RateLimit
	loadedAt   time.Time
	expiry     time.Time
}

type tokenEntry struct {
	timestamp int64
	count     int
}

type slidingWindow struct {
	mu       sync.Mutex
	minute   []int64
	daily    []int64
	tokens   []tokenEntry
}

func NewRateLimiter(cfg *RateLimitConfig) *RateLimiter {
	if cfg == nil {
		cfg = &RateLimitConfig{}
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 5 * time.Minute
	}

	rl := &RateLimiter{
		cfg:    cfg,
		cache:  make(map[string]*cachedLimit),
		window: make(map[string]*slidingWindow),
	}

	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userIDStr := getUserIDFromContext(r.Context())
			if userIDStr == "" {
				next.ServeHTTP(w, r)
				return
			}

			allowed, retryAfter, limitType := rl.checkLimits(userIDStr)
			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]interface{}{
						"message": fmt.Sprintf("Rate limit exceeded: %s", limitType),
						"type":    "rate_limit_exceeded",
						"code":    "rate_limit_exceeded",
					},
					"retry_after": retryAfter,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) checkLimits(userID string) (bool, int, string) {
	limit := rl.getLimit(userID)

	now := time.Now().Unix()
	window := rl.getWindow(userID)
	window.mu.Lock()
	defer window.mu.Unlock()

	if limit.RequestsPerMinute > 0 {
		window.cleanMinute(now)
		minuteCount := countInWindow(window.minute, now, 60)
		if minuteCount >= limit.RequestsPerMinute {
			retry := int(60 - (now - window.minute[0]))
			if retry < 1 {
				retry = 1
			}
			return false, retry, "requests_per_minute"
		}
	}

	if limit.RequestsPerDay > 0 {
		window.cleanDaily(now)
		dayCount := countInWindow(window.daily, now, 86400)
		if dayCount >= limit.RequestsPerDay {
			retry := int(86400 - (now - window.daily[0]))
			if retry < 1 {
				retry = 60
			}
			return false, retry, "requests_per_day"
		}
	}

	if limit.TokensPerMinute > 0 {
		window.cleanTokens(now)
		tokenCount := window.sumTokens(now, 60)
		if tokenCount >= limit.TokensPerMinute {
			retry := int(60 - (now - window.tokens[0].timestamp))
			if retry < 1 {
				retry = 1
			}
			return false, retry, "tokens_per_minute"
		}
	}

	window.minute = append(window.minute, now)
	window.daily = append(window.daily, now)

	return true, 0, ""
}

func (rl *RateLimiter) getLimit(userID string) *db.RateLimit {
	rl.mu.RLock()
	cached, ok := rl.cache[userID]
	if ok && time.Now().Before(cached.expiry) {
		rl.mu.RUnlock()
		return cached.limit
	}
	rl.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var limit *db.RateLimit
	var err error
	if rl.cfg.DB != nil {
		limit, err = rl.cfg.DB.RateLimits().FindByUserID(ctx, userID)
	}
	if err != nil || limit == nil {
		defaultLimit := &db.RateLimit{
			UserID:            userID,
			RequestsPerMinute: rl.cfg.DefaultRPM,
			RequestsPerDay:    rl.cfg.DefaultRPD,
			TokensPerMinute:   rl.cfg.DefaultTPM,
		}
		return defaultLimit
	}

	rl.mu.Lock()
	rl.cache[userID] = &cachedLimit{
		limit:    limit,
		loadedAt: time.Now(),
		expiry:   time.Now().Add(rl.cfg.CacheTTL),
	}
	rl.mu.Unlock()

	return limit
}

func (rl *RateLimiter) getWindow(userID string) *slidingWindow {
	rl.wmu.RLock()
	w, ok := rl.window[userID]
	if ok {
		rl.wmu.RUnlock()
		return w
	}
	rl.wmu.RUnlock()

	rl.wmu.Lock()
	defer rl.wmu.Unlock()

	w, ok = rl.window[userID]
	if ok {
		return w
	}

	w = &slidingWindow{
		minute: make([]int64, 0, 60),
		daily:  make([]int64, 0, 86400),
		tokens: make([]tokenEntry, 0, 60),
	}
	rl.window[userID] = w
	return w
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cfg.CacheTTL)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for userID, cached := range rl.cache {
			if now.After(cached.expiry) {
				delete(rl.cache, userID)
			}
		}
		rl.mu.Unlock()

		rl.wmu.Lock()
		cleanupNow := time.Now().Unix()
		cutoff := cleanupNow - 86400
		for userID, w := range rl.window {
			w.mu.Lock()
			w.minute = filterTimestamps(w.minute, cutoff)
			w.daily = filterTimestamps(w.daily, cutoff)
			w.cleanTokens(cleanupNow)
			if len(w.minute) == 0 && len(w.daily) == 0 && len(w.tokens) == 0 {
				delete(rl.window, userID)
			}
			w.mu.Unlock()
		}
		rl.wmu.Unlock()
	}
}

func (w *slidingWindow) cleanMinute(now int64) {
	cutoff := now - 60
	w.minute = filterTimestamps(w.minute, cutoff)
}

func (w *slidingWindow) cleanDaily(now int64) {
	cutoff := now - 86400
	w.daily = filterTimestamps(w.daily, cutoff)
}

func (w *slidingWindow) cleanTokens(now int64) {
	cutoff := now - 60
	result := make([]tokenEntry, 0, len(w.tokens))
	for _, e := range w.tokens {
		if e.timestamp >= cutoff {
			result = append(result, e)
		}
	}
	w.tokens = result
}

func countInWindow(timestamps []int64, now int64, windowSec int) int {
	cutoff := now - int64(windowSec)
	count := 0
	for _, ts := range timestamps {
		if ts >= cutoff {
			count++
		}
	}
	return count
}

func filterTimestamps(timestamps []int64, cutoff int64) []int64 {
	result := make([]int64, 0, len(timestamps))
	for _, ts := range timestamps {
		if ts >= cutoff {
			result = append(result, ts)
		}
	}
	return result
}

func (w *slidingWindow) sumTokens(now int64, windowSec int) int {
	cutoff := now - int64(windowSec)
	total := 0
	for _, e := range w.tokens {
		if e.timestamp >= cutoff {
			total += e.count
		}
	}
	return total
}

func (rl *RateLimiter) RecordTokenUsage(userID string, count int) {
	window := rl.getWindow(userID)
	window.mu.Lock()
	defer window.mu.Unlock()
	now := time.Now().Unix()
	window.cleanTokens(now)
	window.tokens = append(window.tokens, tokenEntry{timestamp: now, count: count})
}

func getUserIDFromContext(ctx context.Context) string {
	if claims := GetUserClaims(ctx); claims != nil && claims.UserID != "" {
		return claims.UserID
	}

	if v := ctx.Value(ApiKeyValidationKey); v != nil {
		if av, ok := v.(*db.ApiKeyValidation); ok && av.UserID != "" {
			return av.UserID
		}
	}

	if v := ctx.Value(UserContextKey); v != nil {
		if uc, ok := v.(*UserContext); ok && uc.UserID != "" {
			return uc.UserID
		}
	}

	if v := ctx.Value("user_id"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}

	return ""
}

func TrackTokenUsage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}