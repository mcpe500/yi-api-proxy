package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type DatabaseManager interface {
	Driver() string
	Connect() error
	Close() error
	Migrate() error

	Users() UserRepository
	ApiKeys() ApiKeyRepository
	Providers() ProviderRepository
	Models() ModelRepository
	Combos() ComboRepository
	UsageEvents() UsageRepository
	AuditLogs() AuditRepository
	Settings() SettingsRepository
	Quotas() QuotaRepository
	RateLimits() RateLimitRepository
	Aliases() ModelAliasRepository
}

func NewDatabaseManager(driver, dsn string) (DatabaseManager, error) {
	switch driver {
	case "json":
		return newJSONDriver(dsn)
	case "sqlite":
		return newSQLiteDriver(dsn)
	case "postgres", "postgresql":
		return newPostgresDriver(dsn)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

type UserFilter struct {
	Role   string
	Status string
	Limit  int
	Offset int
}

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	List(ctx context.Context, filter UserFilter) ([]*User, error)
	Update(ctx context.Context, user *User) error
	UpdateStatus(ctx context.Context, id, status string) error
	UpdatePassword(ctx context.Context, id, hash string) error
	UpdateLastLogin(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	CountActiveAdmins(ctx context.Context) (int, error)
}

type ApiKeyRepository interface {
	Create(ctx context.Context, key *ApiKey) error
	FindByID(ctx context.Context, id string) (*ApiKey, error)
	FindByHash(ctx context.Context, hash string) (*ApiKey, error)
	FindByUserID(ctx context.Context, userID string) ([]*ApiKey, error)
	List(ctx context.Context) ([]*ApiKey, error)
	Update(ctx context.Context, key *ApiKey) error
	UpdateLastUsed(ctx context.Context, id string) error
	Revoke(ctx context.Context, id string) error
	RevokeAllForUser(ctx context.Context, userID string) error
	CountByUser(ctx context.Context, userID string) (int, error)
}

type ProviderRepository interface {
	Create(ctx context.Context, conn *ProviderConnection) error
	FindByID(ctx context.Context, id string) (*ProviderConnection, error)
	FindByProvider(ctx context.Context, provider string) ([]*ProviderConnection, error)
	List(ctx context.Context) ([]*ProviderConnection, error)
	Update(ctx context.Context, conn *ProviderConnection) error
	Delete(ctx context.Context, id string) error
	MarkCooldown(ctx context.Context, id string, until int64, errorMsg string) error
	ClearCooldown(ctx context.Context, id string) error
	UpdateStatus(ctx context.Context, id, status string) error
}

type ModelRepository interface {
	Create(ctx context.Context, model *Model) error
	FindByID(ctx context.Context, id string) (*Model, error)
	List(ctx context.Context) ([]*Model, error)
	ListEnabled(ctx context.Context) ([]*Model, error)
	ListByProvider(ctx context.Context, providerID string) ([]*Model, error)
	Update(ctx context.Context, model *Model) error
	Delete(ctx context.Context, id string) error
}

type ComboRepository interface {
	Create(ctx context.Context, combo *Combo) error
	FindByID(ctx context.Context, id string) (*Combo, error)
	FindByName(ctx context.Context, name string) (*Combo, error)
	FindByUser(ctx context.Context, userID string) ([]*Combo, error)
	List(ctx context.Context) ([]*Combo, error)
	ListEnabled(ctx context.Context) ([]*Combo, error)
	Update(ctx context.Context, combo *Combo) error
	Delete(ctx context.Context, id string) error
	AddItem(ctx context.Context, item *ComboItem) error
	RemoveItem(ctx context.Context, itemID string) error
	ReorderItems(ctx context.Context, comboID string, itemIDs []string) error
}

type UsageRepository interface {
	Create(ctx context.Context, event *UsageEvent) error
	FindByID(ctx context.Context, id string) (*UsageEvent, error)
	FindByUserID(ctx context.Context, userID string, limit int) ([]*UsageEvent, error)
	FindByDateRange(ctx context.Context, userID string, from, to int64) ([]*UsageEvent, error)
	GetSummary(ctx context.Context, userID string, from, to int64) (*UsageSummary, error)
	Cleanup(ctx context.Context, before int64) (int64, error)
}

type AuditRepository interface {
	Create(ctx context.Context, log *AuditLog) error
	FindByID(ctx context.Context, id string) (*AuditLog, error)
	List(ctx context.Context, filter *AuditFilter) ([]*AuditLog, int64, error)
}

type SettingsRepository interface {
	Get(ctx context.Context) (*Settings, error)
	Update(ctx context.Context, s *Settings) error
}

type RateLimitRepository interface {
	Create(ctx context.Context, rl *RateLimit) error
	FindByID(ctx context.Context, id string) (*RateLimit, error)
	FindByUserID(ctx context.Context, userID string) (*RateLimit, error)
	List(ctx context.Context) ([]*RateLimit, error)
	Update(ctx context.Context, rl *RateLimit) error
	Delete(ctx context.Context, id string) error
}

type QuotaRepository interface {
	Create(ctx context.Context, quota *Quota) error
	FindByID(ctx context.Context, id string) (*Quota, error)
	FindByUserID(ctx context.Context, userID string) (*Quota, error)
	List(ctx context.Context) ([]*Quota, error)
	Update(ctx context.Context, quota *Quota) error
	IncrementUsage(ctx context.Context, userID string, tokens int64, cost float64) error
	ResetMonthly(ctx context.Context, userID string) error
	Delete(ctx context.Context, id string) error
}

type ModelAliasRepository interface {
	Create(ctx context.Context, alias *ModelAlias) error
	FindByID(ctx context.Context, id string) (*ModelAlias, error)
	FindByName(ctx context.Context, name string) (*ModelAlias, error)
	FindByUser(ctx context.Context, userID string) ([]*ModelAlias, error)
	ListGlobal(ctx context.Context) ([]*ModelAlias, error)
	Update(ctx context.Context, alias *ModelAlias) error
	Delete(ctx context.Context, id string) error
}

type AuditFilter struct {
	ActorID  string
	Action   string
	From     int64
	To       int64
	Limit    int
	Offset   int
}

type UsageSummary struct {
	TotalRequests         int64
	TotalPromptTokens     int64
	TotalCompletionTokens int64
	TotalTokens           int64
	TotalCost             float64
	AvgLatencyMs          float64
	ErrorCount            int64
	TotalLatencyMs        int64
}

type Settings struct {
	RequireLogin           bool
	RequireAPIKey          bool
	EnableRequestBodyLog   bool
	UsageRetentionDays     int
	RequestLogRetentionDays int
	AuditLogRetentionDays  int
}

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
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	KeyHash    string     `json:"key_hash"`
	Scopes     []string   `json:"scopes"`
	Status     string     `json:"status"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type ProviderConnection struct {
	ID              string     `json:"id"`
	Provider        string     `json:"provider"`
	Name            string     `json:"name"`
	AuthType        string     `json:"auth_type"`
	EncryptedSecret []byte      `json:"encrypted_secret,omitempty"`
	BaseURL         string     `json:"base_url,omitempty"`
	Priority        int        `json:"priority"`
	Weight          int        `json:"weight"`
	Status          string     `json:"status"`
	CooldownUntil   *time.Time `json:"cooldown_until,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	LastErrorAt     *time.Time `json:"last_error_at,omitempty"`
	BackoffLevel    int        `json:"backoff_level"`
	CreatedBy       string     `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
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
	Strategy    string      `json:"strategy"`
	IsActive    bool        `json:"is_active"`
	Items       []*ComboItem `json:"items"`
	CreatedBy   string      `json:"created_by"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type ComboItem struct {
	ID             string `json:"id"`
	ComboID        string `json:"combo_id"`
	ProviderID     string `json:"provider_id"`
	ModelID        string `json:"model_id"`
	Priority       int    `json:"priority"`
	Weight         int    `json:"weight"`
	MaxRetries     int    `json:"max_retries"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	Conditions     string `json:"conditions,omitempty"`
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
	UserRole  string   `json:"user_role"`
	KeyID     string   `json:"key_id"`
	KeyPrefix string   `json:"key_prefix"`
	Scopes    []string `json:"scopes"`
}

var DefaultScopes = []string{"chat", "completion", "embedding"}

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

type Quota struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	MonthlyTokenCap  int64     `json:"monthly_token_cap"`
	MonthlyCostCap   float64   `json:"monthly_cost_cap"`
	UsedTokens       int64     `json:"used_tokens"`
	UsedCost         float64   `json:"used_cost"`
	ResetAt          time.Time `json:"reset_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ModelAlias struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	TargetID    string    `json:"target_id"`
	Provider    string    `json:"provider"`
	Description string    `json:"description,omitempty"`
	UserID      string    `json:"user_id,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type jsonDriver struct {
	mu    sync.RWMutex
	file  string
	store *jsonStore
}

type jsonStore struct {
	Users       map[string]*User               `json:"users,omitempty"`
	ApiKeys     map[string]*ApiKey             `json:"api_keys,omitempty"`
	Providers   map[string]*ProviderConnection `json:"providers,omitempty"`
	Models      map[string]*Model              `json:"models,omitempty"`
	Combos      map[string]*Combo              `json:"combos,omitempty"`
	UsageEvents map[string]*UsageEvent         `json:"usage_events,omitempty"`
	AuditLogs   map[string]*AuditLog           `json:"audit_logs,omitempty"`
	RateLimits  map[string]*RateLimit          `json:"rate_limits,omitempty"`
	Aliases     map[string]*ModelAlias         `json:"aliases,omitempty"`
}

type jsonDB struct {
	*jsonDriver
	users      *jsonUserRepo
	apiKeys    *jsonApiKeyRepo
	providers  *jsonProviderRepo
	models     *jsonModelRepo
	combos     *jsonComboRepo
	usage      *jsonUsageRepo
	audit      *jsonAuditRepo
	settings   *jsonSettingsRepo
	rateLimits *jsonRateLimitRepo
	aliases    *jsonAliasRepo
}

func newJSONDriver(dsn string) (DatabaseManager, error) {
	j := &jsonDriver{
		file:  dsn,
		store: &jsonStore{},
	}

	if data, err := os.ReadFile(dsn); err == nil {
		if err := json.Unmarshal(data, j.store); err != nil {
			j.store = &jsonStore{}
		}
	}

	db := &jsonDB{jsonDriver: j}
	db.users = newJSONUserRepo(j)
	db.apiKeys = newJsonApiKeyRepo(j)
	db.providers = newJsonProviderRepo(j)
	db.models = newJsonModelRepo(j)
	db.combos = newJsonComboRepo(j)
	db.usage = newJsonUsageRepo(j)
	db.audit = newJsonAuditRepo(j)
	db.settings = newJsonSettingsRepo(j)
	db.rateLimits = newJsonRateLimitRepo(j)
	db.aliases = newJsonAliasRepo(j)

	return db, nil
}

func (db *jsonDB) Driver() string   { return "json" }
func (db *jsonDB) Connect() error   { return nil }
func (db *jsonDB) Close() error     { return db.jsonDriver.save() }
func (db *jsonDB) Migrate() error   { return nil }

func (db *jsonDB) Users() UserRepository     { return db.users }
func (db *jsonDB) ApiKeys() ApiKeyRepository { return db.apiKeys }
func (db *jsonDB) Providers() ProviderRepository { return db.providers }
func (db *jsonDB) Models() ModelRepository   { return db.models }
func (db *jsonDB) Combos() ComboRepository   { return db.combos }
func (db *jsonDB) UsageEvents() UsageRepository { return db.usage }
func (db *jsonDB) AuditLogs() AuditRepository { return db.audit }
func (db *jsonDB) Settings() SettingsRepository { return db.settings }
func (db *jsonDB) RateLimits() RateLimitRepository { return db.rateLimits }
func (db *jsonDB) Quotas() QuotaRepository          { return nil }
func (db *jsonDB) Aliases() ModelAliasRepository   { return db.aliases }

func (j *jsonDriver) save() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	data, err := json.MarshalIndent(j.store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(j.file, data, 0644)
}

func (j *jsonDriver) load() *jsonStore {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if j.store == nil {
		return &jsonStore{}
	}
	cp := *j.store
	return &cp
}

type jsonUserRepo struct {
	driver *jsonDriver
}

func newJSONUserRepo(d *jsonDriver) *jsonUserRepo {
	return &jsonUserRepo{driver: d}
}

func (r *jsonUserRepo) init() {
	store := r.driver.load()
	if store.Users == nil {
		store.Users = make(map[string]*User)
		r.driver.mu.Lock()
		r.driver.store.Users = store.Users
		r.driver.mu.Unlock()
	}
}

func (r *jsonUserRepo) Create(ctx context.Context, user *User) error {
	r.init()
	store := r.driver.load()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	store.Users[user.ID] = user
	r.driver.mu.Lock()
	r.driver.store.Users = store.Users
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonUserRepo) FindByID(ctx context.Context, id string) (*User, error) {
	store := r.driver.load()
	if store.Users == nil {
		return nil, nil
	}
	return store.Users[id], nil
}

func (r *jsonUserRepo) FindByEmail(ctx context.Context, email string) (*User, error) {
	store := r.driver.load()
	if store.Users == nil {
		return nil, nil
	}
	for _, u := range store.Users {
		if u.Email == email && u.Status != "deleted" {
			return u, nil
		}
	}
	return nil, nil
}

func (r *jsonUserRepo) List(ctx context.Context, filter UserFilter) ([]*User, error) {
	store := r.driver.load()
	var result []*User
	for _, u := range store.Users {
		if u.Status == "deleted" {
			continue
		}
		if filter.Role != "" && u.Role != filter.Role {
			continue
		}
		if filter.Status != "" && u.Status != filter.Status {
			continue
		}
		result = append(result, u)
	}
	return result, nil
}

func (r *jsonUserRepo) Update(ctx context.Context, user *User) error {
	r.init()
	store := r.driver.load()
	user.UpdatedAt = time.Now()
	store.Users[user.ID] = user
	r.driver.mu.Lock()
	r.driver.store.Users = store.Users
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonUserRepo) UpdateStatus(ctx context.Context, id, status string) error {
	user, err := r.FindByID(ctx, id)
	if err != nil || user == nil {
		return err
	}
	user.Status = status
	user.UpdatedAt = time.Now()
	return r.Update(ctx, user)
}

func (r *jsonUserRepo) UpdatePassword(ctx context.Context, id, hash string) error {
	user, err := r.FindByID(ctx, id)
	if err != nil || user == nil {
		return err
	}
	user.PasswordHash = hash
	user.UpdatedAt = time.Now()
	return r.Update(ctx, user)
}

func (r *jsonUserRepo) UpdateLastLogin(ctx context.Context, id string) error {
	user, err := r.FindByID(ctx, id)
	if err != nil || user == nil {
		return err
	}
	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = time.Now()
	return r.Update(ctx, user)
}

func (r *jsonUserRepo) Delete(ctx context.Context, id string) error {
	return r.UpdateStatus(ctx, id, "deleted")
}

func (r *jsonUserRepo) CountActiveAdmins(ctx context.Context) (int, error) {
	store := r.driver.load()
	count := 0
	for _, u := range store.Users {
		if u.Role == "admin" && u.Status == "active" {
			count++
		}
	}
	return count, nil
}

type jsonApiKeyRepo struct {
	driver *jsonDriver
}

func newJsonApiKeyRepo(d *jsonDriver) *jsonApiKeyRepo { return &jsonApiKeyRepo{driver: d} }

func (r *jsonApiKeyRepo) init() {
	store := r.driver.load()
	if store.ApiKeys == nil {
		store.ApiKeys = make(map[string]*ApiKey)
		r.driver.mu.Lock()
		r.driver.store.ApiKeys = store.ApiKeys
		r.driver.mu.Unlock()
	}
}

func (r *jsonApiKeyRepo) Create(ctx context.Context, key *ApiKey) error {
	r.init()
	store := r.driver.load()
	key.CreatedAt = time.Now()
	store.ApiKeys[key.ID] = key
	r.driver.mu.Lock()
	r.driver.store.ApiKeys = store.ApiKeys
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonApiKeyRepo) FindByID(ctx context.Context, id string) (*ApiKey, error) {
	store := r.driver.load()
	return store.ApiKeys[id], nil
}

func (r *jsonApiKeyRepo) FindByHash(ctx context.Context, hash string) (*ApiKey, error) {
	store := r.driver.load()
	for _, k := range store.ApiKeys {
		if k.KeyHash == hash && k.Status == "active" {
			return k, nil
		}
	}
	return nil, nil
}

func (r *jsonApiKeyRepo) FindByUserID(ctx context.Context, userID string) ([]*ApiKey, error) {
	store := r.driver.load()
	var result []*ApiKey
	for _, k := range store.ApiKeys {
		if k.UserID == userID {
			result = append(result, k)
		}
	}
	return result, nil
}

func (r *jsonApiKeyRepo) Update(ctx context.Context, key *ApiKey) error {
	store := r.driver.load()
	store.ApiKeys[key.ID] = key
	r.driver.mu.Lock()
	r.driver.store.ApiKeys = store.ApiKeys
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonApiKeyRepo) UpdateLastUsed(ctx context.Context, id string) error {
	key, _ := r.FindByID(ctx, id)
	if key == nil {
		return nil
	}
	now := time.Now()
	key.LastUsedAt = &now
	return r.Update(ctx, key)
}

func (r *jsonApiKeyRepo) Revoke(ctx context.Context, id string) error {
	key, _ := r.FindByID(ctx, id)
	if key == nil {
		return nil
	}
	key.Status = "revoked"
	now := time.Now()
	key.RevokedAt = &now
	return r.Update(ctx, key)
}

func (r *jsonApiKeyRepo) RevokeAllForUser(ctx context.Context, userID string) error {
	keys, _ := r.FindByUserID(ctx, userID)
	now := time.Now()
	for _, k := range keys {
		if k.Status == "active" {
			k.Status = "revoked"
			k.RevokedAt = &now
		}
	}
	return nil
}

func (r *jsonApiKeyRepo) List(ctx context.Context) ([]*ApiKey, error) {
	store := r.driver.load()
	var result []*ApiKey
	for _, k := range store.ApiKeys {
		result = append(result, k)
	}
	return result, nil
}

func (r *jsonApiKeyRepo) CountByUser(ctx context.Context, userID string) (int, error) {
	keys, _ := r.FindByUserID(ctx, userID)
	return len(keys), nil
}

type jsonProviderRepo struct {
	driver *jsonDriver
}

func newJsonProviderRepo(d *jsonDriver) *jsonProviderRepo { return &jsonProviderRepo{driver: d} }

func (r *jsonProviderRepo) init() {
	store := r.driver.load()
	if store.Providers == nil {
		store.Providers = make(map[string]*ProviderConnection)
		r.driver.mu.Lock()
		r.driver.store.Providers = store.Providers
		r.driver.mu.Unlock()
	}
}

func (r *jsonProviderRepo) Create(ctx context.Context, conn *ProviderConnection) error {
	r.init()
	store := r.driver.load()
	conn.CreatedAt = time.Now()
	conn.UpdatedAt = time.Now()
	store.Providers[conn.ID] = conn
	r.driver.mu.Lock()
	r.driver.store.Providers = store.Providers
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonProviderRepo) FindByID(ctx context.Context, id string) (*ProviderConnection, error) {
	store := r.driver.load()
	return store.Providers[id], nil
}

func (r *jsonProviderRepo) FindByProvider(ctx context.Context, provider string) ([]*ProviderConnection, error) {
	store := r.driver.load()
	var result []*ProviderConnection
	for _, p := range store.Providers {
		if p.Provider == provider {
			result = append(result, p)
		}
	}
	return result, nil
}

func (r *jsonProviderRepo) List(ctx context.Context) ([]*ProviderConnection, error) {
	store := r.driver.load()
	var result []*ProviderConnection
	for _, p := range store.Providers {
		result = append(result, p)
	}
	return result, nil
}

func (r *jsonProviderRepo) Update(ctx context.Context, conn *ProviderConnection) error {
	store := r.driver.load()
	conn.UpdatedAt = time.Now()
	store.Providers[conn.ID] = conn
	r.driver.mu.Lock()
	r.driver.store.Providers = store.Providers
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonProviderRepo) Delete(ctx context.Context, id string) error {
	store := r.driver.load()
	delete(store.Providers, id)
	r.driver.mu.Lock()
	r.driver.store.Providers = store.Providers
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonProviderRepo) MarkCooldown(ctx context.Context, id string, until int64, errorMsg string) error {
	conn, _ := r.FindByID(ctx, id)
	if conn == nil {
		return nil
	}
	conn.Status = "cooldown"
	t := time.Unix(until, 0)
	conn.CooldownUntil = &t
	conn.LastError = errorMsg
	now := time.Now()
	conn.LastErrorAt = &now
	return r.Update(ctx, conn)
}

func (r *jsonProviderRepo) ClearCooldown(ctx context.Context, id string) error {
	conn, _ := r.FindByID(ctx, id)
	if conn == nil {
		return nil
	}
	conn.Status = "active"
	conn.CooldownUntil = nil
	conn.LastError = ""
	conn.LastErrorAt = nil
	conn.BackoffLevel = 0
	return r.Update(ctx, conn)
}

func (r *jsonProviderRepo) UpdateStatus(ctx context.Context, id, status string) error {
	conn, _ := r.FindByID(ctx, id)
	if conn == nil {
		return nil
	}
	conn.Status = status
	conn.UpdatedAt = time.Now()
	return r.Update(ctx, conn)
}

type jsonModelRepo struct {
	driver *jsonDriver
}

func newJsonModelRepo(d *jsonDriver) *jsonModelRepo { return &jsonModelRepo{driver: d} }

func (r *jsonModelRepo) init() {
	store := r.driver.load()
	if store.Models == nil {
		store.Models = make(map[string]*Model)
		r.driver.mu.Lock()
		r.driver.store.Models = store.Models
		r.driver.mu.Unlock()
	}
}

func (r *jsonModelRepo) Create(ctx context.Context, m *Model) error {
	r.init()
	store := r.driver.load()
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()
	store.Models[m.ID] = m
	r.driver.mu.Lock()
	r.driver.store.Models = store.Models
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonModelRepo) FindByID(ctx context.Context, id string) (*Model, error) {
	store := r.driver.load()
	return store.Models[id], nil
}

func (r *jsonModelRepo) List(ctx context.Context) ([]*Model, error) {
	store := r.driver.load()
	var result []*Model
	for _, m := range store.Models {
		result = append(result, m)
	}
	return result, nil
}

func (r *jsonModelRepo) ListEnabled(ctx context.Context) ([]*Model, error) {
	store := r.driver.load()
	var result []*Model
	for _, m := range store.Models {
		if m.Enabled {
			result = append(result, m)
		}
	}
	return result, nil
}

func (r *jsonModelRepo) ListByProvider(ctx context.Context, providerID string) ([]*Model, error) {
	store := r.driver.load()
	var result []*Model
	for _, m := range store.Models {
		if m.ProviderID == providerID || m.Provider == providerID {
			result = append(result, m)
		}
	}
	return result, nil
}

func (r *jsonModelRepo) Update(ctx context.Context, m *Model) error {
	store := r.driver.load()
	m.UpdatedAt = time.Now()
	store.Models[m.ID] = m
	r.driver.mu.Lock()
	r.driver.store.Models = store.Models
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonModelRepo) Delete(ctx context.Context, id string) error {
	store := r.driver.load()
	delete(store.Models, id)
	r.driver.mu.Lock()
	r.driver.store.Models = store.Models
	r.driver.mu.Unlock()
	return r.driver.save()
}

type jsonComboRepo struct {
	driver *jsonDriver
}

func newJsonComboRepo(d *jsonDriver) *jsonComboRepo { return &jsonComboRepo{driver: d} }

func (r *jsonComboRepo) init() {
	store := r.driver.load()
	if store.Combos == nil {
		store.Combos = make(map[string]*Combo)
		r.driver.mu.Lock()
		r.driver.store.Combos = store.Combos
		r.driver.mu.Unlock()
	}
}

func (r *jsonComboRepo) Create(ctx context.Context, c *Combo) error {
	r.init()
	store := r.driver.load()
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	store.Combos[c.ID] = c
	r.driver.mu.Lock()
	r.driver.store.Combos = store.Combos
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonComboRepo) FindByID(ctx context.Context, id string) (*Combo, error) {
	store := r.driver.load()
	return store.Combos[id], nil
}

func (r *jsonComboRepo) FindByName(ctx context.Context, name string) (*Combo, error) {
	store := r.driver.load()
	for _, c := range store.Combos {
		if c.Name == name {
			return c, nil
		}
	}
	return nil, nil
}

func (r *jsonComboRepo) List(ctx context.Context) ([]*Combo, error) {
	store := r.driver.load()
	var result []*Combo
	for _, c := range store.Combos {
		result = append(result, c)
	}
	return result, nil
}

func (r *jsonComboRepo) ListEnabled(ctx context.Context) ([]*Combo, error) {
	store := r.driver.load()
	var result []*Combo
	for _, c := range store.Combos {
		if c.IsActive {
			result = append(result, c)
		}
	}
	return result, nil
}

func (r *jsonComboRepo) Update(ctx context.Context, c *Combo) error {
	store := r.driver.load()
	c.UpdatedAt = time.Now()
	store.Combos[c.ID] = c
	r.driver.mu.Lock()
	r.driver.store.Combos = store.Combos
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonComboRepo) FindByUser(ctx context.Context, userID string) ([]*Combo, error) {
	store := r.driver.load()
	var result []*Combo
	for _, c := range store.Combos {
		if c.CreatedBy == userID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (r *jsonComboRepo) Delete(ctx context.Context, id string) error {
	store := r.driver.load()
	delete(store.Combos, id)
	r.driver.mu.Lock()
	r.driver.store.Combos = store.Combos
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonComboRepo) AddItem(ctx context.Context, item *ComboItem) error {
	combo, err := r.FindByID(ctx, item.ComboID)
	if err != nil || combo == nil {
		return fmt.Errorf("combo not found: %s", item.ComboID)
	}
	combo.Items = append(combo.Items, item)
	return r.Update(ctx, combo)
}

func (r *jsonComboRepo) RemoveItem(ctx context.Context, itemID string) error {
	store := r.driver.load()
	for _, combo := range store.Combos {
		for i, item := range combo.Items {
			if item.ID == itemID {
				combo.Items = append(combo.Items[:i], combo.Items[i+1:]...)
				return r.Update(ctx, combo)
			}
		}
	}
	return nil
}

func (r *jsonComboRepo) ReorderItems(ctx context.Context, comboID string, itemIDs []string) error {
	combo, err := r.FindByID(ctx, comboID)
	if err != nil || combo == nil {
		return fmt.Errorf("combo not found: %s", comboID)
	}
	itemMap := make(map[string]*ComboItem, len(combo.Items))
	for _, item := range combo.Items {
		itemMap[item.ID] = item
	}
	reordered := make([]*ComboItem, 0, len(itemIDs))
	for i, id := range itemIDs {
		item, ok := itemMap[id]
		if !ok {
			return fmt.Errorf("item not found: %s", id)
		}
		item.Priority = i + 1
		reordered = append(reordered, item)
	}
	combo.Items = reordered
	return r.Update(ctx, combo)
}

type jsonUsageRepo struct {
	driver *jsonDriver
}

func newJsonUsageRepo(d *jsonDriver) *jsonUsageRepo { return &jsonUsageRepo{driver: d} }

func (r *jsonUsageRepo) init() {
	store := r.driver.load()
	if store.UsageEvents == nil {
		store.UsageEvents = make(map[string]*UsageEvent)
		r.driver.mu.Lock()
		r.driver.store.UsageEvents = store.UsageEvents
		r.driver.mu.Unlock()
	}
}

func (r *jsonUsageRepo) Create(ctx context.Context, e *UsageEvent) error {
	r.init()
	store := r.driver.load()
	e.CreatedAt = time.Now()
	store.UsageEvents[e.ID] = e
	r.driver.mu.Lock()
	r.driver.store.UsageEvents = store.UsageEvents
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonUsageRepo) FindByID(ctx context.Context, id string) (*UsageEvent, error) {
	store := r.driver.load()
	return store.UsageEvents[id], nil
}

func (r *jsonUsageRepo) FindByUserID(ctx context.Context, userID string, limit int) ([]*UsageEvent, error) {
	store := r.driver.load()
	var result []*UsageEvent
	for _, e := range store.UsageEvents {
		if e.UserID == userID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (r *jsonUsageRepo) FindByDateRange(ctx context.Context, userID string, from, to int64) ([]*UsageEvent, error) {
	events, _ := r.FindByUserID(ctx, userID, 0)
	return events, nil
}

func (r *jsonUsageRepo) GetSummary(ctx context.Context, userID string, from, to int64) (*UsageSummary, error) {
	return &UsageSummary{}, nil
}

func (r *jsonUsageRepo) Cleanup(ctx context.Context, before int64) (int64, error) {
	return 0, nil
}

type jsonAuditRepo struct {
	driver *jsonDriver
}

func newJsonAuditRepo(d *jsonDriver) *jsonAuditRepo { return &jsonAuditRepo{driver: d} }

func (r *jsonAuditRepo) init() {
	store := r.driver.load()
	if store.AuditLogs == nil {
		store.AuditLogs = make(map[string]*AuditLog)
		r.driver.mu.Lock()
		r.driver.store.AuditLogs = store.AuditLogs
		r.driver.mu.Unlock()
	}
}

func (r *jsonAuditRepo) Create(ctx context.Context, l *AuditLog) error {
	r.init()
	store := r.driver.load()
	l.CreatedAt = time.Now()
	store.AuditLogs[l.ID] = l
	r.driver.mu.Lock()
	r.driver.store.AuditLogs = store.AuditLogs
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonAuditRepo) FindByID(ctx context.Context, id string) (*AuditLog, error) {
	store := r.driver.load()
	return store.AuditLogs[id], nil
}

func (r *jsonAuditRepo) List(ctx context.Context, filter *AuditFilter) ([]*AuditLog, int64, error) {
	store := r.driver.load()
	var result []*AuditLog
	for _, l := range store.AuditLogs {
		result = append(result, l)
	}
	return result, int64(len(result)), nil
}

type jsonSettingsRepo struct {
	driver *jsonDriver
}

func newJsonSettingsRepo(d *jsonDriver) *jsonSettingsRepo { return &jsonSettingsRepo{driver: d} }

func (r *jsonSettingsRepo) Get(ctx context.Context) (*Settings, error) {
	return &Settings{}, nil
}

func (r *jsonSettingsRepo) Update(ctx context.Context, s *Settings) error {
	return nil
}

type jsonRateLimitRepo struct {
	driver *jsonDriver
}

func newJsonRateLimitRepo(d *jsonDriver) *jsonRateLimitRepo { return &jsonRateLimitRepo{driver: d} }

func (r *jsonRateLimitRepo) init() {
	store := r.driver.load()
	if store.RateLimits == nil {
		store.RateLimits = make(map[string]*RateLimit)
		r.driver.mu.Lock()
		r.driver.store.RateLimits = store.RateLimits
		r.driver.mu.Unlock()
	}
}

func (r *jsonRateLimitRepo) Create(ctx context.Context, rl *RateLimit) error {
	r.init()
	store := r.driver.load()
	rl.CreatedAt = time.Now()
	rl.UpdatedAt = time.Now()
	store.RateLimits[rl.ID] = rl
	r.driver.mu.Lock()
	r.driver.store.RateLimits = store.RateLimits
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonRateLimitRepo) FindByID(ctx context.Context, id string) (*RateLimit, error) {
	store := r.driver.load()
	if store.RateLimits == nil {
		return nil, nil
	}
	return store.RateLimits[id], nil
}

func (r *jsonRateLimitRepo) FindByUserID(ctx context.Context, userID string) (*RateLimit, error) {
	store := r.driver.load()
	if store.RateLimits == nil {
		return nil, nil
	}
	for _, rl := range store.RateLimits {
		if rl.UserID == userID {
			return rl, nil
		}
	}
	return nil, nil
}

func (r *jsonRateLimitRepo) List(ctx context.Context) ([]*RateLimit, error) {
	store := r.driver.load()
	var result []*RateLimit
	for _, rl := range store.RateLimits {
		result = append(result, rl)
	}
	return result, nil
}

func (r *jsonRateLimitRepo) Update(ctx context.Context, rl *RateLimit) error {
	r.init()
	store := r.driver.load()
	rl.UpdatedAt = time.Now()
	store.RateLimits[rl.ID] = rl
	r.driver.mu.Lock()
	r.driver.store.RateLimits = store.RateLimits
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonRateLimitRepo) Delete(ctx context.Context, id string) error {
	store := r.driver.load()
	delete(store.RateLimits, id)
	r.driver.mu.Lock()
	r.driver.store.RateLimits = store.RateLimits
	r.driver.mu.Unlock()
	return r.driver.save()
}

type postgresDriver struct {
	db *sql.DB
}

func newPostgresDriver(dsn string) (DatabaseManager, error) {
	return nil, nil
}

func (p *postgresDriver) Driver() string            { return "postgres" }
func (p *postgresDriver) Connect() error            { return nil }
func (p *postgresDriver) Close() error              { return nil }
func (p *postgresDriver) Migrate() error            { return nil }
func (p *postgresDriver) Users() UserRepository     { return nil }
func (p *postgresDriver) ApiKeys() ApiKeyRepository { return nil }
func (p *postgresDriver) Providers() ProviderRepository { return nil }
func (p *postgresDriver) Models() ModelRepository   { return nil }
func (p *postgresDriver) Combos() ComboRepository   { return nil }
func (p *postgresDriver) UsageEvents() UsageRepository { return nil }
func (p *postgresDriver) AuditLogs() AuditRepository { return nil }
func (p *postgresDriver) Settings() SettingsRepository { return nil }
func (p *postgresDriver) RateLimits() RateLimitRepository { return nil }
func (p *postgresDriver) Quotas() QuotaRepository          { return nil }
func (p *postgresDriver) Aliases() ModelAliasRepository   { return nil }

type jsonAliasRepo struct {
	driver *jsonDriver
}

func newJsonAliasRepo(d *jsonDriver) *jsonAliasRepo { return &jsonAliasRepo{driver: d} }

func (r *jsonAliasRepo) init() {
	store := r.driver.load()
	if store.Aliases == nil {
		store.Aliases = make(map[string]*ModelAlias)
		r.driver.mu.Lock()
		r.driver.store.Aliases = store.Aliases
		r.driver.mu.Unlock()
	}
}

func (r *jsonAliasRepo) Create(ctx context.Context, alias *ModelAlias) error {
	r.init()
	store := r.driver.load()
	alias.CreatedAt = time.Now()
	store.Aliases[alias.ID] = alias
	r.driver.mu.Lock()
	r.driver.store.Aliases = store.Aliases
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonAliasRepo) FindByID(ctx context.Context, id string) (*ModelAlias, error) {
	store := r.driver.load()
	if store.Aliases == nil {
		return nil, nil
	}
	return store.Aliases[id], nil
}

func (r *jsonAliasRepo) FindByName(ctx context.Context, name string) (*ModelAlias, error) {
	store := r.driver.load()
	if store.Aliases == nil {
		return nil, nil
	}
	for _, a := range store.Aliases {
		if a.Name == name {
			return a, nil
		}
	}
	return nil, nil
}

func (r *jsonAliasRepo) FindByUser(ctx context.Context, userID string) ([]*ModelAlias, error) {
	store := r.driver.load()
	var result []*ModelAlias
	for _, a := range store.Aliases {
		if a.UserID == userID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (r *jsonAliasRepo) ListGlobal(ctx context.Context) ([]*ModelAlias, error) {
	store := r.driver.load()
	var result []*ModelAlias
	for _, a := range store.Aliases {
		if a.UserID == "" {
			result = append(result, a)
		}
	}
	return result, nil
}

func (r *jsonAliasRepo) Update(ctx context.Context, alias *ModelAlias) error {
	store := r.driver.load()
	store.Aliases[alias.ID] = alias
	r.driver.mu.Lock()
	r.driver.store.Aliases = store.Aliases
	r.driver.mu.Unlock()
	return r.driver.save()
}

func (r *jsonAliasRepo) Delete(ctx context.Context, id string) error {
	store := r.driver.load()
	delete(store.Aliases, id)
	r.driver.mu.Lock()
	r.driver.store.Aliases = store.Aliases
	r.driver.mu.Unlock()
	return r.driver.save()
}