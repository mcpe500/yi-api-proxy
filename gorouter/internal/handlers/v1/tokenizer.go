package v1

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorouter/gorouter/internal/caveman"
	"github.com/gorouter/gorouter/internal/rtk"
	"github.com/gorouter/gorouter/internal/translator"
)

type TokenOptimizerConfig struct {
	RTKEnabled       bool
	RTKDefaultFilter string
	CavemanLevel     string
}

type TokenOptimizer struct {
	config TokenOptimizerConfig
	logger *slog.Logger
}

func NewTokenOptimizer(cfg TokenOptimizerConfig, logger *slog.Logger) *TokenOptimizer {
	return &TokenOptimizer{config: cfg, logger: logger}
}

func (to *TokenOptimizer) ShouldCompress(r *http.Request) bool {
	if r.Header.Get("X-RTK") == "true" {
		return true
	}
	return to.config.RTKEnabled
}

func (to *TokenOptimizer) RTKFilter(r *http.Request) string {
	if filter := r.Header.Get("X-RTK-Filter"); filter != "" {
		return strings.ToLower(filter)
	}
	if to.config.RTKDefaultFilter != "" {
		return strings.ToLower(to.config.RTKDefaultFilter)
	}
	return "autodetect"
}

func (to *TokenOptimizer) CavemanLevel(r *http.Request) string {
	if level := r.Header.Get("X-Caveman"); level != "" {
		return strings.ToLower(level)
	}
	return strings.ToLower(to.config.CavemanLevel)
}

func (to *TokenOptimizer) compressContent(r *http.Request, content string) (string, int) {
	if to.RTKFilter(r) == "autodetect" {
		return rtk.Compress(content)
	}
	return rtk.CompressWithFilter(content, to.RTKFilter(r))
}

func (to *TokenOptimizer) ApplyToMessages(r *http.Request, messages []translator.NormalizedMessage) ([]translator.NormalizedMessage, int) {
	totalSaved := 0
	shouldCompress := to.ShouldCompress(r)
	level := to.CavemanLevel(r)

	if !shouldCompress && level == "" {
		return messages, 0
	}

	for i, m := range messages {
		content := m.Content
		if shouldCompress {
			compressed, saved := to.compressContent(r, content)
			totalSaved += saved
			content = compressed
		}
		messages[i].Content = content
	}

	if level != "" {
		messages = caveman.Inject(messages, level)
	}

	if totalSaved > 0 && to.logger != nil {
		to.logger.Info("token optimization applied",
			"savings_bytes", totalSaved,
			"messages", len(messages),
		)
	}

	return messages, totalSaved
}

func (to *TokenOptimizer) ApplyToChatMessages(r *http.Request, msgs []ChatMessage) ([]ChatMessage, int) {
	totalSaved := 0
	shouldCompress := to.ShouldCompress(r)
	level := to.CavemanLevel(r)

	if !shouldCompress && level == "" {
		return msgs, 0
	}

	for i, m := range msgs {
		content := m.Content
		if shouldCompress {
			compressed, saved := to.compressContent(r, content)
			totalSaved += saved
			content = compressed
		}
		msgs[i].Content = content
	}

	if level != "" {
		if prompt := caveman.GetPrompt(level); prompt != "" {
			msgs = append([]ChatMessage{{Role: "system", Content: prompt}}, msgs...)
		}
	}

	if totalSaved > 0 && to.logger != nil {
		to.logger.Info("token optimization applied",
			"savings_bytes", totalSaved,
			"messages", len(msgs),
		)
	}

	return msgs, totalSaved
}

func (to *TokenOptimizer) ApplyToClaudeMessages(r *http.Request, msgs []ClaudeMessage, system interface{}) ([]ClaudeMessage, interface{}, int) {
	totalSaved := 0
	shouldCompress := to.ShouldCompress(r)
	level := to.CavemanLevel(r)

	if !shouldCompress && level == "" {
		return msgs, system, 0
	}

	for i, m := range msgs {
		if s, ok := m.Content.(string); ok {
			content := s
			if shouldCompress {
				compressed, saved := to.compressContent(r, content)
				totalSaved += saved
				content = compressed
			}
			msgs[i].Content = content
		}
	}

	if system != nil {
		if s, ok := system.(string); ok {
			content := s
			if shouldCompress {
				compressed, saved := to.compressContent(r, content)
				totalSaved += saved
				content = compressed
			}
			system = content
		}
	}

	if level != "" {
		if prompt := caveman.GetPrompt(level); prompt != "" {
			if s, ok := system.(string); ok && s != "" {
				system = prompt + "\n\n" + s
			} else if system == nil {
				system = prompt
			}
		}
	}

	if totalSaved > 0 && to.logger != nil {
		to.logger.Info("token optimization applied",
			"savings_bytes", totalSaved,
			"messages", len(msgs),
		)
	}

	return msgs, system, totalSaved
}

func isValidCavemanLevel(level string) bool {
	switch strings.ToLower(level) {
	case "lite", "full", "ultra":
		return true
	default:
		return false
	}
}
