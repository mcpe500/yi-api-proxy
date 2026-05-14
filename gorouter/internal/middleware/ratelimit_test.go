package middleware

import "testing"

func TestRateLimiter(t *testing.T) {
	cfg := &RateLimitConfig{
		DefaultRPM: 2,
		DefaultRPD: 10,
		DefaultTPM: 100,
	}
	rl := NewRateLimiter(cfg)

	t.Run("Minute window", func(t *testing.T) {
		userID := "user1"
		
		// 1st request
		ok, _, _ := rl.checkLimits(userID)
		if !ok { t.Error("expected 1st request to be allowed") }

		// 2nd request
		ok, _, _ = rl.checkLimits(userID)
		if !ok { t.Error("expected 2nd request to be allowed") }

		// 3rd request - should be blocked
		ok, _, _ = rl.checkLimits(userID)
		if ok { t.Error("expected 3rd request to be blocked") }
	})

	t.Run("Token tracking", func(t *testing.T) {
		userID := "user2"
		rl.RecordTokenUsage(userID, 101)
		
		ok, _, reason := rl.checkLimits(userID)
		if ok || reason != "tokens_per_minute" {
			t.Errorf("expected token limit block, got %v (%s)", ok, reason)
		}
	})
}
