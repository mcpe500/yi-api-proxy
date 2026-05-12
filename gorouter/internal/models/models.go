package db

import "time"

type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	PasswordHash string     `json:"password_hash"`
	CreatedBy    *string    `json:"created_by,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

type ApiKey struct {
	ID          string     `json:"id"`
	Key         string     `json:"key,omitempty"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	KeyPrefix   string     `json:"key_prefix"`
	KeyHash     string     `json:"key_hash"`
	Scopes      []string   `json:"scopes"`
	Status      string     `json:"status"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

type ApiKeyCreated struct {
	ID        string     `json:"id"`
	Key       string     `json:"key"`
	Name      string     `json:"name"`
	KeyPrefix string     `json:"key_prefix"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type ApiKeyValidation struct {
	UserID    string   `json:"user_id"`
	UserRole   string   `json:"user_role"`
	KeyID     string   `json:"key_id"`
	KeyPrefix string   `json:"key_prefix"`
	Scopes    []string `json:"scopes"`
}

const (
	ScopeChat       = "chat"
	ScopeResponses  = "responses"
	ScopeEmbeddings = "embeddings"
	ScopeImages     = "images"
	ScopeAudio      = "audio"
	ScopeModelsRead = "models:read"
)

var DefaultScopes = []string{ScopeChat, ScopeModelsRead}

type ProviderConnection struct {
	ID               string     `json:"id"`
	Provider         string     `json:"provider"`
	Name             string     `json:"name"`
	AuthType         string     `json:"auth_type"`
	EncryptedSecret  []byte     `json:"encrypted_secret,omitempty"`
	BaseURL          string     `json:"base_url,omitempty"`
	Priority         int        `json:"priority"`
	Weight           int        `json:"weight"`
	Status           string     `json:"status"`
	CooldownUntil    *time.Time `json:"cooldown_until,omitempty"`
	LastError        string     `json:"last_error,omitempty"`
	LastErrorAt      *time.Time `json:"last_error_at,omitempty"`
	BackoffLevel     int        `json:"backoff_level"`
	CreatedBy        string     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type Model struct {
	ID               string    `json:"id"`
	ProviderID       string    `json:"provider_id"`
	ModelID          string    `json:"model_id"`
	DisplayName      string    `json:"display_name"`
	Provider         string    `json:"provider"`
	ModelName        string    `json:"model_name"`
	Mode             string    `json:"mode"`
	Capabilities     []string  `json:"capabilities"`
	ContextWindow    int       `json:"context_window"`
	MaxOutputTokens  int       `json:"max_output_tokens"`
	InputPrice       float64   `json:"input_price"`
	OutputPrice      float64   `json:"output_price"`
	InputCostPer1k   float64   `json:"input_cost_per_1k"`
	OutputCostPer1k  float64   `json:"output_cost_per_1k"`
	Enabled          bool      `json:"enabled"`
	IsActive         bool      `json:"is_active"`
	Tags             []string  `json:"tags"`
	DefaultTimeoutMs int       `json:"default_timeout_ms"`
	SupportsStream   bool      `json:"supports_stream"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Combo struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	UserID      string      `json:"user_id"`
	IsActive    bool        `json:"is_active"`
	Items       []*ComboItem `json:"items"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type ComboItem struct {
	ID             string `json:"id"`
	ComboID        string `json:"combo_id"`
	ProviderID     string `json:"provider_id"`
	ModelID        string `json:"model_id"`
	Priority       int    `json:"priority"`
	MaxRetries     int    `json:"max_retries"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type UsageSummary struct {
	TotalRequests       int64   `json:"total_requests"`
	TotalPromptTokens   int64   `json:"total_prompt_tokens"`
	TotalCompletionTokens int64 `json:"total_completion_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	TotalCost           float64 `json:"total_cost"`
	TotalLatencyMs      int64   `json:"total_latency_ms"`
	AvgLatencyMs        float64 `json:"avg_latency_ms"`
	ErrorCount          int64   `json:"error_count"`
}

type UsageEvent struct {
	ID               string    `json:"id"`
	RequestID        string    `json:"request_id"`
	UserID           string    `json:"user_id"`
	ApiKeyID         string    `json:"api_key_id"`
	RequestedModel   string    `json:"requested_model"`
	FinalModel       string    `json:"final_model"`
	Provider         string    `json:"provider"`
	StatusCode       int       `json:"status_code"`
	ErrorCode        string    `json:"error_code,omitempty"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	EstimatedCost    float64   `json:"estimated_cost"`
	LatencyMs        int       `json:"latency_ms"`
	FallbackCount    int       `json:"fallback_count"`
	IsStream         bool      `json:"is_stream"`
	CreatedAt        time.Time `json:"created_at"`
}

func (s *UsageSummary) AggregatedByUser(events []*UsageEvent) *UsageSummary {
	if s == nil {
		s = &UsageSummary{}
	}
	for _, e := range events {
		s.TotalRequests++
		s.TotalPromptTokens += int64(e.PromptTokens)
		s.TotalCompletionTokens += int64(e.CompletionTokens)
		s.TotalTokens += int64(e.TotalTokens)
		s.TotalCost += e.EstimatedCost
		s.TotalLatencyMs += int64(e.LatencyMs)
		if e.StatusCode >= 400 {
			s.ErrorCount++
		}
	}
	if s.TotalRequests > 0 {
		s.AvgLatencyMs = float64(s.TotalLatencyMs) / float64(s.TotalRequests)
	}
	return s
}

type Quota struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	MonthlyLimit   int64     `json:"monthly_limit"`
	UsedThisMonth  int64     `json:"used_this_month"`
	ResetAt        time.Time `json:"reset_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (q *Quota) Remaining() int64 {
	return q.MonthlyLimit - q.UsedThisMonth
}

func (q *Quota) IsExceeded() bool {
	return q.UsedThisMonth >= q.MonthlyLimit
}

type AuditLog struct {
	ID         string    `json:"id"`
	ActorID    string    `json:"actor_id"`
	ActorEmail string    `json:"actor_email"`
	ActorRole  string    `json:"actor_role"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Details    string    `json:"details,omitempty"`
	IPAddress  string    `json:"ip_address,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type RateLimit struct {
	ID                string    `json:"id"`
	UserID            string    `json:"user_id"`
	RequestsPerMinute int       `json:"requests_per_minute"`
	RequestsPerDay    int       `json:"requests_per_day"`
	TokensPerMinute   int       `json:"tokens_per_minute"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
