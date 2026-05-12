package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type pgDB struct {
	driver *pgDriver
	users      *pgUserRepo
	apiKeys    *pgApiKeyRepo
	providers  *pgProviderRepo
	models     *pgModelRepo
	combos     *pgComboRepo
	usage      *pgUsageRepo
	audit      *pgAuditRepo
	settings   *pgSettingsRepo
	rateLimits *pgRateLimitRepo
	quotas     *pgQuotaRepo
	aliases    *pgAliasRepo
}

func newPgDriver(dsn string) (DatabaseManager, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	d := &pgDriver{db: db}
	sdb := &pgDB{driver: d}
	sdb.users = &pgUserRepo{driver: d}
	sdb.apiKeys = &pgApiKeyRepo{driver: d}
	sdb.providers = &pgProviderRepo{driver: d}
	sdb.models = &pgModelRepo{driver: d}
	sdb.combos = &pgComboRepo{driver: d}
	sdb.usage = &pgUsageRepo{driver: d}
	sdb.audit = &pgAuditRepo{driver: d}
	sdb.settings = &pgSettingsRepo{driver: d}
	sdb.rateLimits = &pgRateLimitRepo{driver: d}
	sdb.quotas = &pgQuotaRepo{driver: d}
	sdb.aliases = &pgAliasRepo{driver: d}

	return sdb, nil
}

func (s *pgDB) Driver() string                { return "postgres" }
func (s *pgDB) Connect() error                { return s.driver.db.PingContext(context.Background()) }
func (s *pgDB) Close() error                  { return s.driver.db.Close() }
func (s *pgDB) Users() UserRepository          { return s.users }
func (s *pgDB) ApiKeys() ApiKeyRepository      { return s.apiKeys }
func (s *pgDB) Providers() ProviderRepository  { return s.providers }
func (s *pgDB) Models() ModelRepository        { return s.models }
func (s *pgDB) Combos() ComboRepository        { return s.combos }
func (s *pgDB) UsageEvents() UsageRepository   { return s.usage }
func (s *pgDB) AuditLogs() AuditRepository     { return s.audit }
func (s *pgDB) Settings() SettingsRepository   { return s.settings }
func (s *pgDB) RateLimits() RateLimitRepository { return s.rateLimits }
func (s *pgDB) Quotas() QuotaRepository         { return s.quotas }
func (s *pgDB) Aliases() ModelAliasRepository   { return s.aliases }

func (s *pgDB) Migrate() error {
	return s.driver.migrate()
}

type pgDriver struct {
	db *sql.DB
}

func (d *pgDriver) migrate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL DEFAULT '',
			role TEXT NOT NULL DEFAULT 'user',
			status TEXT NOT NULL DEFAULT 'active',
			password_hash TEXT NOT NULL DEFAULT '',
			created_by TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_login_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_status ON users(status)`,

		`CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			key_prefix TEXT NOT NULL DEFAULT '',
			key_hash TEXT NOT NULL UNIQUE,
			scopes TEXT[] NOT NULL DEFAULT '{}',
			status TEXT NOT NULL DEFAULT 'active',
			expires_at TIMESTAMPTZ,
			last_used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			revoked_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash)`,

		`CREATE TABLE IF NOT EXISTS providers (
			id TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			name TEXT NOT NULL,
			auth_type TEXT NOT NULL DEFAULT 'bearer',
			encrypted_secret BYTEA,
			base_url TEXT NOT NULL DEFAULT '',
			priority INTEGER NOT NULL DEFAULT 0,
			weight INTEGER NOT NULL DEFAULT 100,
			status TEXT NOT NULL DEFAULT 'active',
			last_latency_ms INTEGER NOT NULL DEFAULT 0,
			cooldown_until TIMESTAMPTZ,
			last_error TEXT NOT NULL DEFAULT '',
			last_error_at TIMESTAMPTZ,
			backoff_level INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_providers_status ON providers(status)`,

		`CREATE TABLE IF NOT EXISTS models (
			id TEXT PRIMARY KEY,
			provider_id TEXT NOT NULL,
			model_id TEXT NOT NULL,
			display_name TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			model_name TEXT NOT NULL DEFAULT '',
			mode TEXT NOT NULL DEFAULT 'chat',
			capabilities JSONB NOT NULL DEFAULT '[]',
			context_window INTEGER NOT NULL DEFAULT 0,
			max_output_tokens INTEGER NOT NULL DEFAULT 0,
			input_price REAL NOT NULL DEFAULT 0,
			output_price REAL NOT NULL DEFAULT 0,
			input_cost_per_1k REAL NOT NULL DEFAULT 0,
			output_cost_per_1k REAL NOT NULL DEFAULT 0,
			enabled BOOLEAN NOT NULL DEFAULT true,
			is_active BOOLEAN NOT NULL DEFAULT true,
			tags JSONB NOT NULL DEFAULT '[]',
			default_timeout_ms INTEGER NOT NULL DEFAULT 30000,
			supports_stream BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_models_provider_id ON models(provider_id)`,
		`CREATE INDEX IF NOT EXISTS idx_models_enabled ON models(enabled)`,

		`CREATE TABLE IF NOT EXISTS combos (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			user_id TEXT NOT NULL,
			strategy TEXT NOT NULL DEFAULT 'priority',
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_by TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_combos_user_id ON combos(user_id)`,

		`CREATE TABLE IF NOT EXISTS combo_items (
			id TEXT PRIMARY KEY,
			combo_id TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			model_id TEXT NOT NULL,
			priority INTEGER NOT NULL DEFAULT 0,
			weight INTEGER NOT NULL DEFAULT 100,
			max_retries INTEGER NOT NULL DEFAULT 3,
			timeout_seconds INTEGER NOT NULL DEFAULT 60,
			conditions TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_combo_items_combo_id ON combo_items(combo_id)`,

		`CREATE TABLE IF NOT EXISTS usage_events (
			id TEXT PRIMARY KEY,
			request_id TEXT NOT NULL DEFAULT '',
			user_id TEXT NOT NULL,
			api_key_id TEXT NOT NULL DEFAULT '',
			requested_model TEXT NOT NULL DEFAULT '',
			final_model TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			status_code INTEGER NOT NULL DEFAULT 0,
			error_code TEXT NOT NULL DEFAULT '',
			prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			estimated_cost REAL NOT NULL DEFAULT 0,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			fallback_count INTEGER NOT NULL DEFAULT 0,
			is_stream BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_user_id ON usage_events(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_created_at ON usage_events(created_at)`,

		`CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			actor_id TEXT NOT NULL,
			actor_email TEXT NOT NULL DEFAULT '',
			actor_role TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			target_type TEXT NOT NULL DEFAULT '',
			target_id TEXT NOT NULL DEFAULT '',
			details TEXT NOT NULL DEFAULT '',
			ip_address TEXT NOT NULL DEFAULT '',
			user_agent TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id ON audit_logs(actor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at)`,

		`CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			require_login BOOLEAN NOT NULL DEFAULT false,
			require_api_key BOOLEAN NOT NULL DEFAULT false,
			enable_request_body_log BOOLEAN NOT NULL DEFAULT false,
			usage_retention_days INTEGER NOT NULL DEFAULT 180,
			request_log_retention_days INTEGER NOT NULL DEFAULT 30,
			audit_log_retention_days INTEGER NOT NULL DEFAULT 365
		)`,
		`INSERT INTO settings (id) VALUES (1) ON CONFLICT DO NOTHING`,

		`CREATE TABLE IF NOT EXISTS quotas (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL UNIQUE,
			monthly_token_cap BIGINT NOT NULL DEFAULT 0,
			monthly_cost_cap REAL NOT NULL DEFAULT 0,
			used_tokens BIGINT NOT NULL DEFAULT 0,
			used_cost REAL NOT NULL DEFAULT 0,
			reset_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_quotas_user_id ON quotas(user_id)`,

		`CREATE TABLE IF NOT EXISTS rate_limits (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL UNIQUE,
			requests_per_minute INTEGER NOT NULL DEFAULT 60,
			requests_per_day INTEGER NOT NULL DEFAULT 1000,
			tokens_per_minute INTEGER NOT NULL DEFAULT 100000,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rate_limits_user_id ON rate_limits(user_id)`,

		`CREATE TABLE IF NOT EXISTS model_aliases (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			target_id TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			user_id TEXT NOT NULL DEFAULT '',
			created_by TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_model_aliases_name ON model_aliases(name)`,
		`CREATE INDEX IF NOT EXISTS idx_model_aliases_user_id ON model_aliases(user_id)`,
	}

	for _, stmt := range statements {
		if _, err := d.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration failed: %w\nstmt: %s", err, stmt)
		}
	}

	return nil
}

type pgUserRepo struct{ driver *pgDriver }

func (r *pgUserRepo) Create(ctx context.Context, u *User) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO users (id, email, name, role, status, password_hash, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		u.ID, u.Email, u.Name, u.Role, u.Status, u.PasswordHash, u.CreatedBy, u.CreatedAt, u.UpdatedAt)
	return err
}
func (r *pgUserRepo) FindByID(ctx context.Context, id string) (*User, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,email,name,role,status,password_hash,created_by,created_at,updated_at,last_login_at FROM users WHERE id=$1`, id)
	return r.scanUser(row)
}
func (r *pgUserRepo) FindByEmail(ctx context.Context, email string) (*User, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,email,name,role,status,password_hash,created_by,created_at,updated_at,last_login_at FROM users WHERE email=$1 AND status!='deleted'`, email)
	return r.scanUser(row)
}
func (r *pgUserRepo) List(ctx context.Context, f UserFilter) ([]*User, error) {
	q := `SELECT id,email,name,role,status,password_hash,created_by,created_at,updated_at,last_login_at FROM users WHERE 1=1`
	var args []interface{}
	if f.Role != "" { q += ` AND role=$` + fmt.Sprint(len(args)+1); args = append(args, f.Role) }
	if f.Status != "" { q += ` AND status=$` + fmt.Sprint(len(args)+1); args = append(args, f.Status) }
	q += ` ORDER BY created_at DESC`
	if f.Limit > 0 { q += fmt.Sprintf(` LIMIT %d`, f.Limit) }
	if f.Offset > 0 { q += fmt.Sprintf(` OFFSET %d`, f.Offset) }
	rows, err := r.driver.db.QueryContext(ctx, q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []*User
	for rows.Next() {
		if u, err := r.scanUserRow(rows); err == nil { result = append(result, u) }
	}
	return result, rows.Err()
}
func (r *pgUserRepo) Update(ctx context.Context, u *User) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE users SET email=$1,name=$2,role=$3,status=$4,password_hash=$5,updated_at=$6,last_login_at=$7 WHERE id=$8`,
		u.Email, u.Name, u.Role, u.Status, u.PasswordHash, u.UpdatedAt, u.LastLoginAt, u.ID)
	return err
}
func (r *pgUserRepo) UpdateStatus(ctx context.Context, id, status string) error { _, err := r.driver.db.ExecContext(ctx, `UPDATE users SET status=$1,updated_at=NOW() WHERE id=$2`, status, id); return err }
func (r *pgUserRepo) UpdatePassword(ctx context.Context, id, hash string) error { _, err := r.driver.db.ExecContext(ctx, `UPDATE users SET password_hash=$1,updated_at=NOW() WHERE id=$2`, hash, id); return err }
func (r *pgUserRepo) UpdateLastLogin(ctx context.Context, id string) error { _, err := r.driver.db.ExecContext(ctx, `UPDATE users SET last_login_at=NOW(),updated_at=NOW() WHERE id=$1`, id); return err }
func (r *pgUserRepo) Delete(ctx context.Context, id string) error { _, err := r.driver.db.ExecContext(ctx, `UPDATE users SET status='deleted',updated_at=NOW() WHERE id=$1`, id); return err }
func (r *pgUserRepo) CountActiveAdmins(ctx context.Context) (int, error) {
	var cnt int
	err := r.driver.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role='admin' AND status='active'`).Scan(&cnt)
	return cnt, err
}
func (r *pgUserRepo) scanUser(row *sql.Row) (*User, error) {
	u := &User{}
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Status, &u.PasswordHash, &u.CreatedBy, &u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt)
	if err == sql.ErrNoRows { return nil, nil }
	return u, err
}
func (r *pgUserRepo) scanUserRow(rows *sql.Rows) (*User, error) {
	u := &User{}
	err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Status, &u.PasswordHash, &u.CreatedBy, &u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt)
	return u, err
}

type pgApiKeyRepo struct{ driver *pgDriver }

func (r *pgApiKeyRepo) Create(ctx context.Context, k *ApiKey) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO api_keys (id,user_id,name,key_prefix,key_hash,scopes,status,expires_at,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		k.ID, k.UserID, k.Name, k.KeyPrefix, k.KeyHash, k.Scopes, k.Status, k.ExpiresAt, k.CreatedAt)
	return err
}
func (r *pgApiKeyRepo) FindByID(ctx context.Context, id string) (*ApiKey, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,user_id,name,key_prefix,key_hash,scopes,status,expires_at,last_used_at,created_at,revoked_at FROM api_keys WHERE id=$1`, id)
	return r.scanApiKey(row)
}
func (r *pgApiKeyRepo) FindByHash(ctx context.Context, hash string) (*ApiKey, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,user_id,name,key_prefix,key_hash,scopes,status,expires_at,last_used_at,created_at,revoked_at FROM api_keys WHERE key_hash=$1 AND status='active'`, hash)
	return r.scanApiKey(row)
}
func (r *pgApiKeyRepo) FindByUserID(ctx context.Context, userID string) ([]*ApiKey, error) {
	rows, err := r.driver.db.QueryContext(ctx, `SELECT id,user_id,name,key_prefix,key_hash,scopes,status,expires_at,last_used_at,created_at,revoked_at FROM api_keys WHERE user_id=$1`, userID)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []*ApiKey
	for rows.Next() { if k, err := r.scanApiKeyRows(rows); err == nil { result = append(result, k) } }
	return result, rows.Err()
}
func (r *pgApiKeyRepo) List(ctx context.Context) ([]*ApiKey, error) {
	rows, err := r.driver.db.QueryContext(ctx, `SELECT id,user_id,name,key_prefix,key_hash,scopes,status,expires_at,last_used_at,created_at,revoked_at FROM api_keys`)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []*ApiKey
	for rows.Next() { if k, err := r.scanApiKeyRows(rows); err == nil { result = append(result, k) } }
	return result, rows.Err()
}
func (r *pgApiKeyRepo) Update(ctx context.Context, k *ApiKey) error {
	_, err := r.driver.db.ExecContext(ctx, `UPDATE api_keys SET name=$1,scopes=$2,status=$3,expires_at=$4,last_used_at=$5,revoked_at=$6 WHERE id=$7`,
		k.Name, k.Scopes, k.Status, k.ExpiresAt, k.LastUsedAt, k.RevokedAt, k.ID)
	return err
}
func (r *pgApiKeyRepo) UpdateLastUsed(ctx context.Context, id string) error { _, err := r.driver.db.ExecContext(ctx, `UPDATE api_keys SET last_used_at=NOW() WHERE id=$1`, id); return err }
func (r *pgApiKeyRepo) Revoke(ctx context.Context, id string) error { _, err := r.driver.db.ExecContext(ctx, `UPDATE api_keys SET status='revoked',revoked_at=NOW() WHERE id=$1`, id); return err }
func (r *pgApiKeyRepo) RevokeAllForUser(ctx context.Context, userID string) error { _, err := r.driver.db.ExecContext(ctx, `UPDATE api_keys SET status='revoked',revoked_at=NOW() WHERE user_id=$1 AND status='active'`, userID); return err }
func (r *pgApiKeyRepo) CountByUser(ctx context.Context, userID string) (int, error) {
	var cnt int
	err := r.driver.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys WHERE user_id=$1`, userID).Scan(&cnt)
	return cnt, err
}
func (r *pgApiKeyRepo) scanApiKey(row *sql.Row) (*ApiKey, error) {
	k := &ApiKey{}
	err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.Scopes, &k.Status, &k.ExpiresAt, &k.LastUsedAt, &k.CreatedAt, &k.RevokedAt)
	if err == sql.ErrNoRows { return nil, nil }
	return k, err
}
func (r *pgApiKeyRepo) scanApiKeyRows(rows *sql.Rows) (*ApiKey, error) {
	k := &ApiKey{}
	err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.Scopes, &k.Status, &k.ExpiresAt, &k.LastUsedAt, &k.CreatedAt, &k.RevokedAt)
	return k, err
}

type pgProviderRepo struct{ driver *pgDriver }

func (r *pgProviderRepo) Create(ctx context.Context, p *ProviderConnection) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO providers (id,provider,name,auth_type,encrypted_secret,base_url,priority,weight,status,last_latency_ms,cooldown_until,last_error,last_error_at,backoff_level,created_by,created_at,updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		p.ID, p.Provider, p.Name, p.AuthType, p.EncryptedSecret, p.BaseURL, p.Priority, p.Weight, p.Status, p.LastLatencyMs, p.CooldownUntil, p.LastError, p.LastErrorAt, p.BackoffLevel, p.CreatedBy, p.CreatedAt, p.UpdatedAt)
	return err
}
func (r *pgProviderRepo) FindByID(ctx context.Context, id string) (*ProviderConnection, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id,provider,name,auth_type,encrypted_secret,base_url,priority,weight,status,last_latency_ms,cooldown_until,last_error,last_error_at,backoff_level,created_by,created_at,updated_at FROM providers WHERE id=$1`, id)
	return r.scan(row)
}
func (r *pgProviderRepo) FindByProvider(ctx context.Context, provider string) ([]*ProviderConnection, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id,provider,name,auth_type,encrypted_secret,base_url,priority,weight,status,last_latency_ms,cooldown_until,last_error,last_error_at,backoff_level,created_by,created_at,updated_at FROM providers WHERE provider=$1`, provider)
	if err != nil { return nil, err }
	defer rows.Close()
	return r.scanAll(rows)
}
func (r *pgProviderRepo) List(ctx context.Context) ([]*ProviderConnection, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id,provider,name,auth_type,encrypted_secret,base_url,priority,weight,status,last_latency_ms,cooldown_until,last_error,last_error_at,backoff_level,created_by,created_at,updated_at FROM providers ORDER BY priority DESC,name`)
	if err != nil { return nil, err }
	defer rows.Close()
	return r.scanAll(rows)
}
func (r *pgProviderRepo) Update(ctx context.Context, p *ProviderConnection) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE providers SET name=$1,base_url=$2,priority=$3,weight=$4,status=$5,last_latency_ms=$6,encrypted_secret=$7,cooldown_until=$8,last_error=$9,last_error_at=$10,backoff_level=$11,updated_at=$12 WHERE id=$13`,
		p.Name, p.BaseURL, p.Priority, p.Weight, p.Status, p.LastLatencyMs, p.EncryptedSecret, p.CooldownUntil, p.LastError, p.LastErrorAt, p.BackoffLevel, p.UpdatedAt, p.ID)
	return err
}
func (r *pgProviderRepo) Delete(ctx context.Context, id string) error { _, err := r.driver.db.ExecContext(ctx, `DELETE FROM providers WHERE id=$1`, id); return err }
func (r *pgProviderRepo) MarkCooldown(ctx context.Context, id string, until int64, errMsg string) error {
	_, err := r.driver.db.ExecContext(ctx, `UPDATE providers SET status='cooldown',cooldown_until=TO_TIMESTAMP($1),last_error=$2,last_error_at=NOW(),backoff_level=backoff_level+1 WHERE id=$3`, until, errMsg, id)
	return err
}
func (r *pgProviderRepo) ClearCooldown(ctx context.Context, id string) error { _, err := r.driver.db.ExecContext(ctx, `UPDATE providers SET status='active',cooldown_until=NULL,last_error='',last_error_at=NULL,backoff_level=0 WHERE id=$1`, id); return err }
func (r *pgProviderRepo) UpdateStatus(ctx context.Context, id, status string) error { _, err := r.driver.db.ExecContext(ctx, `UPDATE providers SET status=$1,updated_at=NOW() WHERE id=$2`, status, id); return err }
func (r *pgProviderRepo) scan(row *sql.Row) (*ProviderConnection, error) {
	p := &ProviderConnection{}
	err := row.Scan(&p.ID, &p.Provider, &p.Name, &p.AuthType, &p.EncryptedSecret, &p.BaseURL, &p.Priority, &p.Weight, &p.Status, &p.LastLatencyMs, &p.CooldownUntil, &p.LastError, &p.LastErrorAt, &p.BackoffLevel, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows { return nil, nil }
	return p, err
}
func (r *pgProviderRepo) scanAll(rows *sql.Rows) ([]*ProviderConnection, error) {
	var result []*ProviderConnection
	for rows.Next() {
		p := &ProviderConnection{}
		if err := rows.Scan(&p.ID, &p.Provider, &p.Name, &p.AuthType, &p.EncryptedSecret, &p.BaseURL, &p.Priority, &p.Weight, &p.Status, &p.LastLatencyMs, &p.CooldownUntil, &p.LastError, &p.LastErrorAt, &p.BackoffLevel, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err == nil {
			result = append(result, p)
		}
	}
	return result, rows.Err()
}

type pgModelRepo struct{ driver *pgDriver }

func (r *pgModelRepo) Create(ctx context.Context, m *Model) error {
	caps, _ := json.Marshal(m.Capabilities)
	tags, _ := json.Marshal(m.Tags)
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO models (id,provider_id,model_id,display_name,provider,model_name,mode,capabilities,context_window,max_output_tokens,input_price,output_price,input_cost_per_1k,output_cost_per_1k,enabled,is_active,tags,default_timeout_ms,supports_stream,created_at,updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		m.ID, m.ProviderID, m.ModelID, m.DisplayName, m.Provider, m.ModelName, m.Mode, caps, m.ContextWindow, m.MaxOutputTokens, m.InputPrice, m.OutputPrice, m.InputCostPer1k, m.OutputCostPer1k, m.Enabled, m.IsActive, tags, m.DefaultTimeoutMs, m.SupportsStream, m.CreatedAt, m.UpdatedAt)
	return err
}
func (r *pgModelRepo) FindByID(ctx context.Context, id string) (*Model, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,provider_id,model_id,display_name,provider,model_name,mode,capabilities,context_window,max_output_tokens,input_price,output_price,input_cost_per_1k,output_cost_per_1k,enabled,is_active,tags,default_timeout_ms,supports_stream,created_at,updated_at FROM models WHERE id=$1`, id)
	return r.scan(row)
}
func (r *pgModelRepo) List(ctx context.Context) ([]*Model, error) {
	rows, err := r.driver.db.QueryContext(ctx, `SELECT id,provider_id,model_id,display_name,provider,model_name,mode,capabilities,context_window,max_output_tokens,input_price,output_price,input_cost_per_1k,output_cost_per_1k,enabled,is_active,tags,default_timeout_ms,supports_stream,created_at,updated_at FROM models`)
	if err != nil { return nil, err }
	defer rows.Close()
	return r.scanAll(rows)
}
func (r *pgModelRepo) ListEnabled(ctx context.Context) ([]*Model, error) {
	rows, err := r.driver.db.QueryContext(ctx, `SELECT id,provider_id,model_id,display_name,provider,model_name,mode,capabilities,context_window,max_output_tokens,input_price,output_price,input_cost_per_1k,output_cost_per_1k,enabled,is_active,tags,default_timeout_ms,supports_stream,created_at,updated_at FROM models WHERE enabled=true`)
	if err != nil { return nil, err }
	defer rows.Close()
	return r.scanAll(rows)
}
func (r *pgModelRepo) ListByProvider(ctx context.Context, providerID string) ([]*Model, error) {
	rows, err := r.driver.db.QueryContext(ctx, `SELECT id,provider_id,model_id,display_name,provider,model_name,mode,capabilities,context_window,max_output_tokens,input_price,output_price,input_cost_per_1k,output_cost_per_1k,enabled,is_active,tags,default_timeout_ms,supports_stream,created_at,updated_at FROM models WHERE provider_id=$1`, providerID)
	if err != nil { return nil, err }
	defer rows.Close()
	return r.scanAll(rows)
}
func (r *pgModelRepo) Update(ctx context.Context, m *Model) error {
	caps, _ := json.Marshal(m.Capabilities)
	tags, _ := json.Marshal(m.Tags)
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE models SET provider_id=$1,model_id=$2,display_name=$3,provider=$4,model_name=$5,mode=$6,capabilities=$7,context_window=$8,max_output_tokens=$9,input_price=$10,output_price=$11,input_cost_per_1k=$12,output_cost_per_1k=$13,enabled=$14,is_active=$15,tags=$16,default_timeout_ms=$17,supports_stream=$18,updated_at=$19 WHERE id=$20`,
		m.ProviderID, m.ModelID, m.DisplayName, m.Provider, m.ModelName, m.Mode, caps, m.ContextWindow, m.MaxOutputTokens, m.InputPrice, m.OutputPrice, m.InputCostPer1k, m.OutputCostPer1k, m.Enabled, m.IsActive, tags, m.DefaultTimeoutMs, m.SupportsStream, m.UpdatedAt, m.ID)
	return err
}
func (r *pgModelRepo) Delete(ctx context.Context, id string) error { _, err := r.driver.db.ExecContext(ctx, `DELETE FROM models WHERE id=$1`, id); return err }
func (r *pgModelRepo) scan(row *sql.Row) (*Model, error) {
	m := &Model{}
	var capsJSON, tagsJSON string
	err := row.Scan(&m.ID, &m.ProviderID, &m.ModelID, &m.DisplayName, &m.Provider, &m.ModelName, &m.Mode, &capsJSON, &m.ContextWindow, &m.MaxOutputTokens, &m.InputPrice, &m.OutputPrice, &m.InputCostPer1k, &m.OutputCostPer1k, &m.Enabled, &m.IsActive, &tagsJSON, &m.DefaultTimeoutMs, &m.SupportsStream, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows { return nil, nil }
	json.Unmarshal([]byte(capsJSON), &m.Capabilities)
	json.Unmarshal([]byte(tagsJSON), &m.Tags)
	return m, err
}
func (r *pgModelRepo) scanAll(rows *sql.Rows) ([]*Model, error) {
	var result []*Model
	for rows.Next() {
		m := &Model{}
		var capsJSON, tagsJSON string
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.ModelID, &m.DisplayName, &m.Provider, &m.ModelName, &m.Mode, &capsJSON, &m.ContextWindow, &m.MaxOutputTokens, &m.InputPrice, &m.OutputPrice, &m.InputCostPer1k, &m.OutputCostPer1k, &m.Enabled, &m.IsActive, &tagsJSON, &m.DefaultTimeoutMs, &m.SupportsStream, &m.CreatedAt, &m.UpdatedAt); err == nil {
			json.Unmarshal([]byte(capsJSON), &m.Capabilities)
			json.Unmarshal([]byte(tagsJSON), &m.Tags)
			result = append(result, m)
		}
	}
	return result, rows.Err()
}

type pgComboRepo struct{ driver *pgDriver }

func (r *pgComboRepo) Create(ctx context.Context, c *Combo) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO combos (id,name,description,user_id,strategy,is_active,created_by,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		c.ID, c.Name, c.Description, c.UserID, c.Strategy, c.IsActive, c.CreatedBy, c.CreatedAt, c.UpdatedAt)
	return err
}
func (r *pgComboRepo) FindByID(ctx context.Context, id string) (*Combo, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,name,description,user_id,strategy,is_active,created_by,created_at,updated_at FROM combos WHERE id=$1`, id)
	c := &Combo{}
	err := row.Scan(&c.ID, &c.Name, &c.Description, &c.UserID, &c.Strategy, &c.IsActive, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows { return nil, nil }
	return c, err
}
func (r *pgComboRepo) FindByName(ctx context.Context, name string) (*Combo, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,name,description,user_id,strategy,is_active,created_by,created_at,updated_at FROM combos WHERE name=$1`, name)
	c := &Combo{}
	err := row.Scan(&c.ID, &c.Name, &c.Description, &c.UserID, &c.Strategy, &c.IsActive, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows { return nil, nil }
	return c, err
}
func (r *pgComboRepo) FindByUser(ctx context.Context, userID string) ([]*Combo, error) {
	rows, err := r.driver.db.QueryContext(ctx, `SELECT id,name,description,user_id,strategy,is_active,created_by,created_at,updated_at FROM combos WHERE user_id=$1`, userID)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []*Combo
	for rows.Next() { c := &Combo{}; if rows.Scan(&c.ID, &c.Name, &c.Description, &c.UserID, &c.Strategy, &c.IsActive, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt) == nil { result = append(result, c) } }
	return result, rows.Err()
}
func (r *pgComboRepo) List(ctx context.Context) ([]*Combo, error) {
	rows, err := r.driver.db.QueryContext(ctx, `SELECT id,name,description,user_id,strategy,is_active,created_by,created_at,updated_at FROM combos`)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []*Combo
	for rows.Next() { c := &Combo{}; if rows.Scan(&c.ID, &c.Name, &c.Description, &c.UserID, &c.Strategy, &c.IsActive, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt) == nil { result = append(result, c) } }
	return result, rows.Err()
}
func (r *pgComboRepo) ListEnabled(ctx context.Context) ([]*Combo, error) {
	rows, err := r.driver.db.QueryContext(ctx, `SELECT id,name,description,user_id,strategy,is_active,created_by,created_at,updated_at FROM combos WHERE is_active=true`)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []*Combo
	for rows.Next() { c := &Combo{}; if rows.Scan(&c.ID, &c.Name, &c.Description, &c.UserID, &c.Strategy, &c.IsActive, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt) == nil { result = append(result, c) } }
	return result, rows.Err()
}
func (r *pgComboRepo) Update(ctx context.Context, c *Combo) error { _, err := r.driver.db.ExecContext(ctx, `UPDATE combos SET name=$1,description=$2,strategy=$3,is_active=$4,updated_at=$5 WHERE id=$6`, c.Name, c.Description, c.Strategy, c.IsActive, c.UpdatedAt, c.ID); return err }
func (r *pgComboRepo) Delete(ctx context.Context, id string) error { _, err := r.driver.db.ExecContext(ctx, `DELETE FROM combos WHERE id=$1`, id); return err }
func (r *pgComboRepo) AddItem(ctx context.Context, item *ComboItem) error {
	_, err := r.driver.db.ExecContext(ctx, `INSERT INTO combo_items (id,combo_id,provider_id,model_id,priority,weight,max_retries,timeout_seconds,conditions) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		item.ID, item.ComboID, item.ProviderID, item.ModelID, item.Priority, item.Weight, item.MaxRetries, item.TimeoutSeconds, item.Conditions)
	return err
}
func (r *pgComboRepo) RemoveItem(ctx context.Context, itemID string) error { _, err := r.driver.db.ExecContext(ctx, `DELETE FROM combo_items WHERE id=$1`, itemID); return err }
func (r *pgComboRepo) ReorderItems(ctx context.Context, comboID string, itemIDs []string) error {
	for i, id := range itemIDs { r.driver.db.ExecContext(ctx, `UPDATE combo_items SET priority=$1 WHERE id=$2 AND combo_id=$3`, i+1, id, comboID) }
	return nil
}

type pgUsageRepo struct{ driver *pgDriver }

func (r *pgUsageRepo) Create(ctx context.Context, e *UsageEvent) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO usage_events (id,request_id,user_id,api_key_id,requested_model,final_model,provider,status_code,error_code,prompt_tokens,completion_tokens,total_tokens,estimated_cost,latency_ms,fallback_count,is_stream,created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		e.ID, e.RequestID, e.UserID, e.ApiKeyID, e.RequestedModel, e.FinalModel, e.Provider, e.StatusCode, e.ErrorCode, e.PromptTokens, e.CompletionTokens, e.TotalTokens, e.EstimatedCost, e.LatencyMs, e.FallbackCount, e.IsStream, e.CreatedAt)
	return err
}
func (r *pgUsageRepo) FindByID(ctx context.Context, id string) (*UsageEvent, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,request_id,user_id,api_key_id,requested_model,final_model,provider,status_code,error_code,prompt_tokens,completion_tokens,total_tokens,estimated_cost,latency_ms,fallback_count,is_stream,created_at FROM usage_events WHERE id=$1`, id)
	e := &UsageEvent{}
	err := row.Scan(&e.ID, &e.RequestID, &e.UserID, &e.ApiKeyID, &e.RequestedModel, &e.FinalModel, &e.Provider, &e.StatusCode, &e.ErrorCode, &e.PromptTokens, &e.CompletionTokens, &e.TotalTokens, &e.EstimatedCost, &e.LatencyMs, &e.FallbackCount, &e.IsStream, &e.CreatedAt)
	if err == sql.ErrNoRows { return nil, nil }
	return e, err
}
func (r *pgUsageRepo) FindByUserID(ctx context.Context, userID string, limit int) ([]*UsageEvent, error) {
	q := `SELECT id,request_id,user_id,api_key_id,requested_model,final_model,provider,status_code,error_code,prompt_tokens,completion_tokens,total_tokens,estimated_cost,latency_ms,fallback_count,is_stream,created_at FROM usage_events WHERE user_id=$1 ORDER BY created_at DESC`
	if limit > 0 { q += fmt.Sprintf(` LIMIT %d`, limit) }
	rows, err := r.driver.db.QueryContext(ctx, q, userID)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []*UsageEvent
	for rows.Next() { e := &UsageEvent{}; if rows.Scan(&e.ID, &e.RequestID, &e.UserID, &e.ApiKeyID, &e.RequestedModel, &e.FinalModel, &e.Provider, &e.StatusCode, &e.ErrorCode, &e.PromptTokens, &e.CompletionTokens, &e.TotalTokens, &e.EstimatedCost, &e.LatencyMs, &e.FallbackCount, &e.IsStream, &e.CreatedAt) == nil { result = append(result, e) } }
	return result, rows.Err()
}
func (r *pgUsageRepo) FindByDateRange(ctx context.Context, userID string, from, to int64) ([]*UsageEvent, error) {
	rows, err := r.driver.db.QueryContext(ctx, `SELECT id,request_id,user_id,api_key_id,requested_model,final_model,provider,status_code,error_code,prompt_tokens,completion_tokens,total_tokens,estimated_cost,latency_ms,fallback_count,is_stream,created_at FROM usage_events WHERE user_id=$1 AND created_at BETWEEN TO_TIMESTAMP($2) AND TO_TIMESTAMP($3) ORDER BY created_at DESC`,
		userID, from, to)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []*UsageEvent
	for rows.Next() { e := &UsageEvent{}; if rows.Scan(&e.ID, &e.RequestID, &e.UserID, &e.ApiKeyID, &e.RequestedModel, &e.FinalModel, &e.Provider, &e.StatusCode, &e.ErrorCode, &e.PromptTokens, &e.CompletionTokens, &e.TotalTokens, &e.EstimatedCost, &e.LatencyMs, &e.FallbackCount, &e.IsStream, &e.CreatedAt) == nil { result = append(result, e) } }
	return result, rows.Err()
}
func (r *pgUsageRepo) GetSummary(ctx context.Context, userID string, from, to int64) (*UsageSummary, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT COALESCE(COUNT(*),0),COALESCE(SUM(prompt_tokens),0),COALESCE(SUM(completion_tokens),0),COALESCE(SUM(total_tokens),0),COALESCE(SUM(estimated_cost),0),COALESCE(AVG(latency_ms)::float,0),COALESCE(SUM(CASE WHEN status_code>=500 THEN 1 ELSE 0 END),0) FROM usage_events WHERE user_id=$1 AND created_at BETWEEN TO_TIMESTAMP($2) AND TO_TIMESTAMP($3)`,
		userID, from, to)
	s := &UsageSummary{}
	row.Scan(&s.TotalRequests, &s.TotalPromptTokens, &s.TotalCompletionTokens, &s.TotalTokens, &s.TotalCost, &s.AvgLatencyMs, &s.ErrorCount)
	return s, nil
}
func (r *pgUsageRepo) Cleanup(ctx context.Context, before int64) (int64, error) {
	res, err := r.driver.db.ExecContext(ctx, `DELETE FROM usage_events WHERE created_at < TO_TIMESTAMP($1)`, before)
	if err != nil { return 0, err }
	n, _ := res.RowsAffected()
	return n, nil
}

type pgAuditRepo struct{ driver *pgDriver }

func (r *pgAuditRepo) Create(ctx context.Context, l *AuditLog) error {
	_, err := r.driver.db.ExecContext(ctx, `INSERT INTO audit_logs (id,actor_id,actor_email,actor_role,action,target_type,target_id,details,ip_address,user_agent,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		l.ID, l.ActorID, l.ActorEmail, l.ActorRole, l.Action, l.TargetType, l.TargetID, l.Details, l.IPAddress, l.UserAgent, l.CreatedAt)
	return err
}
func (r *pgAuditRepo) FindByID(ctx context.Context, id string) (*AuditLog, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,actor_id,actor_email,actor_role,action,target_type,target_id,details,ip_address,user_agent,created_at FROM audit_logs WHERE id=$1`, id)
	l := &AuditLog{}
	err := row.Scan(&l.ID, &l.ActorID, &l.ActorEmail, &l.ActorRole, &l.Action, &l.TargetType, &l.TargetID, &l.Details, &l.IPAddress, &l.UserAgent, &l.CreatedAt)
	if err == sql.ErrNoRows { return nil, nil }
	return l, err
}
func (r *pgAuditRepo) List(ctx context.Context, f *AuditFilter) ([]*AuditLog, int64, error) {
	q := `SELECT id,actor_id,actor_email,actor_role,action,target_type,target_id,details,ip_address,user_agent,created_at FROM audit_logs WHERE 1=1`
	var args []interface{}
	if f.ActorID != "" { q += ` AND actor_id=$` + fmt.Sprint(len(args)+1); args = append(args, f.ActorID) }
	if f.Action != "" { q += ` AND action=$` + fmt.Sprint(len(args)+1); args = append(args, f.Action) }
	q += ` ORDER BY created_at DESC`
	if f.Limit > 0 { q += fmt.Sprintf(` LIMIT %d`, f.Limit) }
	if f.Offset > 0 { q += fmt.Sprintf(` OFFSET %d`, f.Offset) }
	rows, err := r.driver.db.QueryContext(ctx, q, args...)
	if err != nil { return nil, 0, err }
	defer rows.Close()
	var result []*AuditLog
	for rows.Next() { l := &AuditLog{}; if rows.Scan(&l.ID, &l.ActorID, &l.ActorEmail, &l.ActorRole, &l.Action, &l.TargetType, &l.TargetID, &l.Details, &l.IPAddress, &l.UserAgent, &l.CreatedAt) == nil { result = append(result, l) } }
	return result, int64(len(result)), rows.Err()
}

type pgSettingsRepo struct{ driver *pgDriver }

func (r *pgSettingsRepo) Get(ctx context.Context) (*Settings, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT require_login,require_api_key,enable_request_body_log,usage_retention_days,request_log_retention_days,audit_log_retention_days FROM settings WHERE id=1`)
	s := &Settings{}
	err := row.Scan(&s.RequireLogin, &s.RequireAPIKey, &s.EnableRequestBodyLog, &s.UsageRetentionDays, &s.RequestLogRetentionDays, &s.AuditLogRetentionDays)
	if err == sql.ErrNoRows { return &Settings{}, nil }
	return s, err
}
func (r *pgSettingsRepo) Update(ctx context.Context, s *Settings) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE settings SET require_login=$1,require_api_key=$2,enable_request_body_log=$3,usage_retention_days=$4,request_log_retention_days=$5,audit_log_retention_days=$6 WHERE id=1`,
		s.RequireLogin, s.RequireAPIKey, s.EnableRequestBodyLog, s.UsageRetentionDays, s.RequestLogRetentionDays, s.AuditLogRetentionDays)
	return err
}

type pgRateLimitRepo struct{ driver *pgDriver }

func (r *pgRateLimitRepo) Create(ctx context.Context, rl *RateLimit) error {
	_, err := r.driver.db.ExecContext(ctx, `INSERT INTO rate_limits (id,user_id,requests_per_minute,requests_per_day,tokens_per_minute,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		rl.ID, rl.UserID, rl.RequestsPerMinute, rl.RequestsPerDay, rl.TokensPerMinute, rl.CreatedAt, rl.UpdatedAt)
	return err
}
func (r *pgRateLimitRepo) FindByID(ctx context.Context, id string) (*RateLimit, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,user_id,requests_per_minute,requests_per_day,tokens_per_minute,created_at,updated_at FROM rate_limits WHERE id=$1`, id)
	rl := &RateLimit{}
	err := row.Scan(&rl.ID, &rl.UserID, &rl.RequestsPerMinute, &rl.RequestsPerDay, &rl.TokensPerMinute, &rl.CreatedAt, &rl.UpdatedAt)
	if err == sql.ErrNoRows { return nil, nil }
	return rl, err
}
func (r *pgRateLimitRepo) FindByUserID(ctx context.Context, userID string) (*RateLimit, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,user_id,requests_per_minute,requests_per_day,tokens_per_minute,created_at,updated_at FROM rate_limits WHERE user_id=$1`, userID)
	rl := &RateLimit{}
	err := row.Scan(&rl.ID, &rl.UserID, &rl.RequestsPerMinute, &rl.RequestsPerDay, &rl.TokensPerMinute, &rl.CreatedAt, &rl.UpdatedAt)
	if err == sql.ErrNoRows { return nil, nil }
	return rl, err
}
func (r *pgRateLimitRepo) List(ctx context.Context) ([]*RateLimit, error) {
	rows, err := r.driver.db.QueryContext(ctx, `SELECT id,user_id,requests_per_minute,requests_per_day,tokens_per_minute,created_at,updated_at FROM rate_limits`)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []*RateLimit
	for rows.Next() { rl := &RateLimit{}; if rows.Scan(&rl.ID, &rl.UserID, &rl.RequestsPerMinute, &rl.RequestsPerDay, &rl.TokensPerMinute, &rl.CreatedAt, &rl.UpdatedAt) == nil { result = append(result, rl) } }
	return result, rows.Err()
}
func (r *pgRateLimitRepo) Update(ctx context.Context, rl *RateLimit) error {
	_, err := r.driver.db.ExecContext(ctx, `UPDATE rate_limits SET requests_per_minute=$1,requests_per_day=$2,tokens_per_minute=$3,updated_at=$4 WHERE id=$5`,
		rl.RequestsPerMinute, rl.RequestsPerDay, rl.TokensPerMinute, rl.UpdatedAt, rl.ID)
	return err
}
func (r *pgRateLimitRepo) Delete(ctx context.Context, id string) error { _, err := r.driver.db.ExecContext(ctx, `DELETE FROM rate_limits WHERE id=$1`, id); return err }

type pgQuotaRepo struct{ driver *pgDriver }

func (r *pgQuotaRepo) Create(ctx context.Context, q *Quota) error {
	_, err := r.driver.db.ExecContext(ctx, `INSERT INTO quotas (id,user_id,monthly_token_cap,monthly_cost_cap,used_tokens,used_cost,reset_at,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		q.ID, q.UserID, q.MonthlyTokenCap, q.MonthlyCostCap, q.UsedTokens, q.UsedCost, q.ResetAt, q.CreatedAt, q.UpdatedAt)
	return err
}
func (r *pgQuotaRepo) FindByID(ctx context.Context, id string) (*Quota, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,user_id,monthly_token_cap,monthly_cost_cap,used_tokens,used_cost,reset_at,created_at,updated_at FROM quotas WHERE id=$1`, id)
	q := &Quota{}
	err := row.Scan(&q.ID, &q.UserID, &q.MonthlyTokenCap, &q.MonthlyCostCap, &q.UsedTokens, &q.UsedCost, &q.ResetAt, &q.CreatedAt, &q.UpdatedAt)
	if err == sql.ErrNoRows { return nil, nil }
	return q, err
}
func (r *pgQuotaRepo) FindByUserID(ctx context.Context, userID string) (*Quota, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,user_id,monthly_token_cap,monthly_cost_cap,used_tokens,used_cost,reset_at,created_at,updated_at FROM quotas WHERE user_id=$1`, userID)
	q := &Quota{}
	err := row.Scan(&q.ID, &q.UserID, &q.MonthlyTokenCap, &q.MonthlyCostCap, &q.UsedTokens, &q.UsedCost, &q.ResetAt, &q.CreatedAt, &q.UpdatedAt)
	if err == sql.ErrNoRows { return nil, nil }
	return q, err
}
func (r *pgQuotaRepo) List(ctx context.Context) ([]*Quota, error) {
	rows, err := r.driver.db.QueryContext(ctx, `SELECT id,user_id,monthly_token_cap,monthly_cost_cap,used_tokens,used_cost,reset_at,created_at,updated_at FROM quotas`)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []*Quota
	for rows.Next() { q := &Quota{}; if rows.Scan(&q.ID, &q.UserID, &q.MonthlyTokenCap, &q.MonthlyCostCap, &q.UsedTokens, &q.UsedCost, &q.ResetAt, &q.CreatedAt, &q.UpdatedAt) == nil { result = append(result, q) } }
	return result, rows.Err()
}
func (r *pgQuotaRepo) Update(ctx context.Context, q *Quota) error {
	_, err := r.driver.db.ExecContext(ctx, `UPDATE quotas SET monthly_token_cap=$1,monthly_cost_cap=$2,used_tokens=$3,used_cost=$4,reset_at=$5,updated_at=$6 WHERE id=$7`,
		q.MonthlyTokenCap, q.MonthlyCostCap, q.UsedTokens, q.UsedCost, q.ResetAt, q.UpdatedAt, q.ID)
	return err
}
func (r *pgQuotaRepo) IncrementUsage(ctx context.Context, userID string, tokens int64, cost float64) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE quotas SET used_tokens=used_tokens+$1,used_cost=used_cost+$2,updated_at=NOW() WHERE user_id=$3`,
		tokens, cost, userID)
	return err
}
func (r *pgQuotaRepo) ResetMonthly(ctx context.Context, userID string) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE quotas SET used_tokens=0,used_cost=0,reset_at=NOW() + INTERVAL '1 month',updated_at=NOW() WHERE user_id=$1`,
		userID)
	return err
}
func (r *pgQuotaRepo) Delete(ctx context.Context, id string) error { _, err := r.driver.db.ExecContext(ctx, `DELETE FROM quotas WHERE id=$1`, id); return err }

type pgAliasRepo struct{ driver *pgDriver }

func (r *pgAliasRepo) Create(ctx context.Context, a *ModelAlias) error {
	_, err := r.driver.db.ExecContext(ctx, `INSERT INTO model_aliases (id,name,target_id,provider,description,user_id,created_by,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		a.ID, a.Name, a.TargetID, a.Provider, a.Description, a.UserID, a.CreatedBy, a.CreatedAt)
	return err
}
func (r *pgAliasRepo) FindByID(ctx context.Context, id string) (*ModelAlias, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,name,target_id,provider,description,user_id,created_by,created_at FROM model_aliases WHERE id=$1`, id)
	a := &ModelAlias{}
	err := row.Scan(&a.ID, &a.Name, &a.TargetID, &a.Provider, &a.Description, &a.UserID, &a.CreatedBy, &a.CreatedAt)
	if err == sql.ErrNoRows { return nil, nil }
	return a, err
}
func (r *pgAliasRepo) FindByName(ctx context.Context, name string) (*ModelAlias, error) {
	row := r.driver.db.QueryRowContext(ctx, `SELECT id,name,target_id,provider,description,user_id,created_by,created_at FROM model_aliases WHERE name=$1`, name)
	a := &ModelAlias{}
	err := row.Scan(&a.ID, &a.Name, &a.TargetID, &a.Provider, &a.Description, &a.UserID, &a.CreatedBy, &a.CreatedAt)
	if err == sql.ErrNoRows { return nil, nil }
	return a, err
}
func (r *pgAliasRepo) FindByUser(ctx context.Context, userID string) ([]*ModelAlias, error) {
	rows, err := r.driver.db.QueryContext(ctx, `SELECT id,name,target_id,provider,description,user_id,created_by,created_at FROM model_aliases WHERE user_id=$1`, userID)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []*ModelAlias
	for rows.Next() { a := &ModelAlias{}; if rows.Scan(&a.ID, &a.Name, &a.TargetID, &a.Provider, &a.Description, &a.UserID, &a.CreatedBy, &a.CreatedAt) == nil { result = append(result, a) } }
	return result, rows.Err()
}
func (r *pgAliasRepo) ListGlobal(ctx context.Context) ([]*ModelAlias, error) {
	rows, err := r.driver.db.QueryContext(ctx, `SELECT id,name,target_id,provider,description,user_id,created_by,created_at FROM model_aliases WHERE user_id=''`)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []*ModelAlias
	for rows.Next() { a := &ModelAlias{}; if rows.Scan(&a.ID, &a.Name, &a.TargetID, &a.Provider, &a.Description, &a.UserID, &a.CreatedBy, &a.CreatedAt) == nil { result = append(result, a) } }
	return result, rows.Err()
}
func (r *pgAliasRepo) Update(ctx context.Context, a *ModelAlias) error {
	_, err := r.driver.db.ExecContext(ctx, `UPDATE model_aliases SET name=$1,target_id=$2,provider=$3,description=$4 WHERE id=$5`, a.Name, a.TargetID, a.Provider, a.Description, a.ID)
	return err
}
func (r *pgAliasRepo) Delete(ctx context.Context, id string) error { _, err := r.driver.db.ExecContext(ctx, `DELETE FROM model_aliases WHERE id=$1`, id); return err }