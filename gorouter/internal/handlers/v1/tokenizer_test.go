package v1

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestApplyToChatMessagesCavemanPrependsSystem(t *testing.T) {
	opt := NewTokenOptimizer(TokenOptimizerConfig{}, nil)
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("X-Caveman", "full")

	msgs, _ := opt.ApplyToChatMessages(r, []ChatMessage{{Role: "user", Content: "Explain this."}})

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" || !strings.Contains(msgs[0].Content, "caveman") {
		t.Fatalf("expected caveman system prompt, got %#v", msgs[0])
	}
	if msgs[1].Role != "user" || msgs[1].Content != "Explain this." {
		t.Fatalf("expected original message preserved, got %#v", msgs[1])
	}
}

func TestApplyToClaudeMessagesCavemanSetsSystem(t *testing.T) {
	opt := NewTokenOptimizer(TokenOptimizerConfig{}, nil)
	r := httptest.NewRequest("POST", "/v1/messages", nil)
	r.Header.Set("X-Caveman", "lite")

	_, system, _ := opt.ApplyToClaudeMessages(r, []ClaudeMessage{{Role: "user", Content: "Explain this."}}, nil)

	s, ok := system.(string)
	if !ok || !strings.Contains(s, "terse") {
		t.Fatalf("expected caveman system string, got %#v", system)
	}
}

func TestApplyToMessagesUsesExplicitRTKFilter(t *testing.T) {
	opt := NewTokenOptimizer(TokenOptimizerConfig{}, nil)
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("X-RTK", "true")
	r.Header.Set("X-RTK-Filter", "network")

	msgs, _ := opt.ApplyToChatMessages(r, []ChatMessage{{Role: "user", Content: "  % Total    % Received % Xferd\n100 1234 100 1234 0 0 10k 0 --:--:-- --:--:-- --:--:-- 10k\n{\"ok\":true}"}})

	if strings.Contains(msgs[0].Content, "% Total") {
		t.Fatalf("expected curl progress removed, got %q", msgs[0].Content)
	}
}
