package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type sqliteDriver struct {
	db *sql.DB
}

type sqliteDB struct {
	driver     *sqliteDriver
	users      *sqliteUserRepo
	apiKeys    *sqliteApiKeyRepo
	providers  *sqliteProviderRepo
	models     *sqliteModelRepo
	combos     *sqliteComboRepo
	usage      *sqliteUsageRepo
	audit      *sqliteAuditRepo
	settings   *sqliteSettingsRepo
	rateLimits *sqliteRateLimitRepo
	quotas     *sqliteQuotaRepo
	aliases    *sqliteAliasRepo
}

func newSQLiteDriver(dsn string) (DatabaseManager, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	d := &sqliteDriver{db: db}
	sdb := &sqliteDB{driver: d}
	sdb.users = &sqliteUserRepo{driver: d}
	sdb.apiKeys = &sqliteApiKeyRepo{driver: d}
	sdb.providers = &sqliteProviderRepo{driver: d}
	sdb.models = &sqliteModelRepo{driver: d}
	sdb.combos = &sqliteComboRepo{driver: d}
	sdb.usage = &sqliteUsageRepo{driver: d}
	sdb.audit = &sqliteAuditRepo{driver: d}
	sdb.settings = &sqliteSettingsRepo{driver: d}
	sdb.rateLimits = &sqliteRateLimitRepo{driver: d}
	sdb.quotas = &sqliteQuotaRepo{driver: d}
	sdb.aliases = &sqliteAliasRepo{driver: d}

	return sdb, nil
}

func (s *sqliteDB) Driver() string                  { return "sqlite" }
func (s *sqliteDB) Connect() error                  { return s.driver.db.PingContext(context.Background()) }
func (s *sqliteDB) Ping() error                     { return s.driver.db.PingContext(context.Background()) }
func (s *sqliteDB) Close() error                    { return s.driver.db.Close() }
func (s *sqliteDB) Users() UserRepository           { return s.users }
func (s *sqliteDB) ApiKeys() ApiKeyRepository       { return s.apiKeys }
func (s *sqliteDB) Providers() ProviderRepository   { return s.providers }
func (s *sqliteDB) Models() ModelRepository         { return s.models }
func (s *sqliteDB) Combos() ComboRepository         { return s.combos }
func (s *sqliteDB) UsageEvents() UsageRepository    { return s.usage }
func (s *sqliteDB) AuditLogs() AuditRepository      { return s.audit }
func (s *sqliteDB) Settings() SettingsRepository    { return s.settings }
func (s *sqliteDB) RateLimits() RateLimitRepository { return s.rateLimits }
func (s *sqliteDB) Quotas() QuotaRepository         { return s.quotas }
func (s *sqliteDB) Aliases() ModelAliasRepository   { return s.aliases }

func (s *sqliteDB) Migrate() error {
	return s.driver.migrate()
}

func (d *sqliteDriver) migrate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_login_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_status ON users(status)`,

		`CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			key_prefix TEXT NOT NULL DEFAULT '',
			key_hash TEXT NOT NULL UNIQUE,
			scopes TEXT NOT NULL DEFAULT '[]',
			status TEXT NOT NULL DEFAULT 'active',
			expires_at DATETIME,
			last_used_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			revoked_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash)`,

		`CREATE TABLE IF NOT EXISTS providers (
			id TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			name TEXT NOT NULL,
			auth_type TEXT NOT NULL DEFAULT 'bearer',
			encrypted_secret BLOB,
			base_url TEXT NOT NULL DEFAULT '',
			priority INTEGER NOT NULL DEFAULT 0,
			weight INTEGER NOT NULL DEFAULT 100,
			status TEXT NOT NULL DEFAULT 'active',
			last_latency_ms INTEGER NOT NULL DEFAULT 0,
			cooldown_until DATETIME,
			last_error TEXT NOT NULL DEFAULT '',
			last_error_at DATETIME,
			backoff_level INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
			capabilities TEXT NOT NULL DEFAULT '[]',
			context_window INTEGER NOT NULL DEFAULT 0,
			max_output_tokens INTEGER NOT NULL DEFAULT 0,
			input_price REAL NOT NULL DEFAULT 0,
			output_price REAL NOT NULL DEFAULT 0,
			input_cost_per_1k REAL NOT NULL DEFAULT 0,
			output_cost_per_1k REAL NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1,
			is_active INTEGER NOT NULL DEFAULT 1,
			tags TEXT NOT NULL DEFAULT '[]',
			default_timeout_ms INTEGER NOT NULL DEFAULT 30000,
			supports_stream INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_models_provider_id ON models(provider_id)`,
		`CREATE INDEX IF NOT EXISTS idx_models_enabled ON models(enabled)`,

		`CREATE TABLE IF NOT EXISTS combos (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			user_id TEXT NOT NULL,
			strategy TEXT NOT NULL DEFAULT 'priority',
			is_active INTEGER NOT NULL DEFAULT 1,
			created_by TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
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
			timeout_seconds INTEGER NOT NULL DEFAULT 30,
			conditions TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (combo_id) REFERENCES combos(id) ON DELETE CASCADE,
			FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
			FOREIGN KEY (model_id) REFERENCES models(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_combo_items_combo_id ON combo_items(combo_id)`,

		`CREATE TABLE IF NOT EXISTS usage_events (
			id TEXT PRIMARY KEY,
			request_id TEXT NOT NULL,
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
			is_stream INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
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
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id ON audit_logs(actor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at)`,

		`CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			require_login INTEGER NOT NULL DEFAULT 0,
			require_api_key INTEGER NOT NULL DEFAULT 0,
			enable_request_body_log INTEGER NOT NULL DEFAULT 0,
			usage_retention_days INTEGER NOT NULL DEFAULT 180,
			request_log_retention_days INTEGER NOT NULL DEFAULT 30,
			audit_log_retention_days INTEGER NOT NULL DEFAULT 365
		)`,
		`INSERT OR IGNORE INTO settings (id) VALUES (1)`,

		`CREATE TABLE IF NOT EXISTS quotas (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL UNIQUE,
			monthly_token_cap INTEGER NOT NULL DEFAULT 0,
			monthly_cost_cap REAL NOT NULL DEFAULT 0,
			used_tokens INTEGER NOT NULL DEFAULT 0,
			used_cost REAL NOT NULL DEFAULT 0,
			reset_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_quotas_user_id ON quotas(user_id)`,

		`CREATE TABLE IF NOT EXISTS rate_limits (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL UNIQUE,
			requests_per_minute INTEGER NOT NULL DEFAULT 60,
			requests_per_day INTEGER NOT NULL DEFAULT 1000,
			tokens_per_minute INTEGER NOT NULL DEFAULT 100000,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
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
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_model_aliases_name ON model_aliases(name)`,
		`CREATE INDEX IF NOT EXISTS idx_model_aliases_user_id ON model_aliases(user_id)`,
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin migration tx: %w", err)
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration failed: %w\nstatement: %s", err, stmt)
		}
	}

	return tx.Commit()
}

type sqliteUserRepo struct{ driver *sqliteDriver }
type sqliteApiKeyRepo struct{ driver *sqliteDriver }
type sqliteProviderRepo struct{ driver *sqliteDriver }
type sqliteModelRepo struct{ driver *sqliteDriver }
type sqliteComboRepo struct{ driver *sqliteDriver }
type sqliteUsageRepo struct{ driver *sqliteDriver }
type sqliteAuditRepo struct{ driver *sqliteDriver }
type sqliteSettingsRepo struct{ driver *sqliteDriver }
type sqliteRateLimitRepo struct{ driver *sqliteDriver }
type sqliteQuotaRepo struct{ driver *sqliteDriver }
type sqliteAliasRepo struct{ driver *sqliteDriver }

func (r *sqliteUserRepo) Create(ctx context.Context, user *User) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO users (id, email, name, role, status, password_hash, created_by, created_at, updated_at, last_login_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Email, user.Name, user.Role, user.Status, user.PasswordHash,
		user.CreatedBy, user.CreatedAt, user.UpdatedAt, user.LastLoginAt)
	return err
}

func (r *sqliteUserRepo) FindByID(ctx context.Context, id string) (*User, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, email, name, role, status, password_hash, created_by, created_at, updated_at, last_login_at
		 FROM users WHERE id = ?`, id)
	return r.scanUser(row)
}

func (r *sqliteUserRepo) FindByEmail(ctx context.Context, email string) (*User, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, email, name, role, status, password_hash, created_by, created_at, updated_at, last_login_at
		 FROM users WHERE email = ? AND status != 'deleted'`, email)
	return r.scanUser(row)
}

func (r *sqliteUserRepo) List(ctx context.Context, filter UserFilter) ([]*User, error) {
	query := `SELECT id, email, name, role, status, password_hash, created_by, created_at, updated_at, last_login_at
			  FROM users WHERE status != 'deleted'`
	args := []interface{}{}
	if filter.Role != "" {
		query += " AND role = ?"
		args = append(args, filter.Role)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.driver.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u, err := r.scanRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *sqliteUserRepo) Update(ctx context.Context, user *User) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE users SET email=?, name=?, role=?, status=?, password_hash=?, updated_at=?, last_login_at=?
		 WHERE id = ?`,
		user.Email, user.Name, user.Role, user.Status, user.PasswordHash,
		user.UpdatedAt, user.LastLoginAt, user.ID)
	return err
}

func (r *sqliteUserRepo) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE users SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id)
	return err
}

func (r *sqliteUserRepo) UpdatePassword(ctx context.Context, id, hash string) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		hash, time.Now(), id)
	return err
}

func (r *sqliteUserRepo) UpdateLastLogin(ctx context.Context, id string) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE users SET last_login_at = ?, updated_at = ? WHERE id = ?`,
		time.Now(), time.Now(), id)
	return err
}

func (r *sqliteUserRepo) Delete(ctx context.Context, id string) error {
	return r.UpdateStatus(ctx, id, "deleted")
}

func (r *sqliteUserRepo) CountActiveAdmins(ctx context.Context) (int, error) {
	var count int
	err := r.driver.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active'`).Scan(&count)
	return count, err
}

func (r *sqliteUserRepo) scanUser(row *sql.Row) (*User, error) {
	u := &User{}
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Status, &u.PasswordHash,
		&u.CreatedBy, &u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (r *sqliteUserRepo) scanRow(rows *sql.Rows) (*User, error) {
	u := &User{}
	err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Status, &u.PasswordHash,
		&u.CreatedBy, &u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt)
	return u, err
}

func (r *sqliteApiKeyRepo) Create(ctx context.Context, key *ApiKey) error {
	scopes, _ := json.Marshal(key.Scopes)
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO api_keys (id, user_id, name, key_prefix, key_hash, scopes, status, expires_at, last_used_at, created_at, revoked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		key.ID, key.UserID, key.Name, key.KeyPrefix, key.KeyHash,
		string(scopes), key.Status, key.ExpiresAt, key.LastUsedAt, key.CreatedAt, key.RevokedAt)
	return err
}

func (r *sqliteApiKeyRepo) FindByID(ctx context.Context, id string) (*ApiKey, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, key_prefix, key_hash, scopes, status, expires_at, last_used_at, created_at, revoked_at
		 FROM api_keys WHERE id = ?`, id)
	return r.scanApiKey(row)
}

func (r *sqliteApiKeyRepo) FindByHash(ctx context.Context, hash string) (*ApiKey, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, key_prefix, key_hash, scopes, status, expires_at, last_used_at, created_at, revoked_at
		 FROM api_keys WHERE key_hash = ? AND status = 'active'`, hash)
	return r.scanApiKey(row)
}

func (r *sqliteApiKeyRepo) FindByUserID(ctx context.Context, userID string) ([]*ApiKey, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, user_id, name, key_prefix, key_hash, scopes, status, expires_at, last_used_at, created_at, revoked_at
		 FROM api_keys WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanApiKeys(rows)
}

func (r *sqliteApiKeyRepo) List(ctx context.Context) ([]*ApiKey, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, user_id, name, key_prefix, key_hash, scopes, status, expires_at, last_used_at, created_at, revoked_at
		 FROM api_keys`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanApiKeys(rows)
}

func (r *sqliteApiKeyRepo) Update(ctx context.Context, key *ApiKey) error {
	scopes, _ := json.Marshal(key.Scopes)
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE api_keys SET name=?, status=?, scopes=?, expires_at=?, last_used_at=?, revoked_at=?
		 WHERE id = ?`,
		key.Name, key.Status, string(scopes), key.ExpiresAt, key.LastUsedAt, key.RevokedAt, key.ID)
	return err
}

func (r *sqliteApiKeyRepo) UpdateLastUsed(ctx context.Context, id string) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = ? WHERE id = ?`, time.Now(), id)
	return err
}

func (r *sqliteApiKeyRepo) Revoke(ctx context.Context, id string) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE api_keys SET status = 'revoked', revoked_at = ? WHERE id = ?`, time.Now(), id)
	return err
}

func (r *sqliteApiKeyRepo) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE api_keys SET status = 'revoked', revoked_at = ? WHERE user_id = ? AND status = 'active'`,
		time.Now(), userID)
	return err
}

func (r *sqliteApiKeyRepo) CountByUser(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.driver.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM api_keys WHERE user_id = ?`, userID).Scan(&count)
	return count, err
}

func (r *sqliteApiKeyRepo) scanApiKey(row *sql.Row) (*ApiKey, error) {
	k := &ApiKey{}
	var scopes string
	err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash,
		&scopes, &k.Status, &k.ExpiresAt, &k.LastUsedAt, &k.CreatedAt, &k.RevokedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(scopes), &k.Scopes)
	return k, nil
}

func (r *sqliteApiKeyRepo) scanApiKeys(rows *sql.Rows) ([]*ApiKey, error) {
	var keys []*ApiKey
	for rows.Next() {
		k := &ApiKey{}
		var scopes string
		err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash,
			&scopes, &k.Status, &k.ExpiresAt, &k.LastUsedAt, &k.CreatedAt, &k.RevokedAt)
		if err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(scopes), &k.Scopes)
		keys = append(keys, k)
	}
	return keys, nil
}

func (r *sqliteProviderRepo) Create(ctx context.Context, conn *ProviderConnection) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO providers (id, provider, name, auth_type, encrypted_secret, base_url, priority, weight, status, last_latency_ms, cooldown_until, last_error, last_error_at, backoff_level, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		conn.ID, conn.Provider, conn.Name, conn.AuthType, conn.EncryptedSecret,
		conn.BaseURL, conn.Priority, conn.Weight, conn.Status, conn.LastLatencyMs,
		conn.CooldownUntil, conn.LastError, conn.LastErrorAt, conn.BackoffLevel,
		conn.CreatedBy, conn.CreatedAt, conn.UpdatedAt)
	return err
}

func (r *sqliteProviderRepo) FindByID(ctx context.Context, id string) (*ProviderConnection, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, provider, name, auth_type, encrypted_secret, base_url, priority, weight, status, last_latency_ms, cooldown_until, last_error, last_error_at, backoff_level, created_by, created_at, updated_at
		 FROM providers WHERE id = ?`, id)
	conn := &ProviderConnection{}
	err := row.Scan(&conn.ID, &conn.Provider, &conn.Name, &conn.AuthType, &conn.EncryptedSecret,
		&conn.BaseURL, &conn.Priority, &conn.Weight, &conn.Status, &conn.LastLatencyMs,
		&conn.CooldownUntil, &conn.LastError, &conn.LastErrorAt, &conn.BackoffLevel,
		&conn.CreatedBy, &conn.CreatedAt, &conn.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return conn, err
}

func (r *sqliteProviderRepo) FindByProvider(ctx context.Context, provider string) ([]*ProviderConnection, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, provider, name, auth_type, encrypted_secret, base_url, priority, weight, status, last_latency_ms, cooldown_until, last_error, last_error_at, backoff_level, created_by, created_at, updated_at
		 FROM providers WHERE provider = ?`, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanProviders(rows)
}

func (r *sqliteProviderRepo) List(ctx context.Context) ([]*ProviderConnection, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, provider, name, auth_type, encrypted_secret, base_url, priority, weight, status, last_latency_ms, cooldown_until, last_error, last_error_at, backoff_level, created_by, created_at, updated_at
		 FROM providers ORDER BY priority DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanProviders(rows)
}

func (r *sqliteProviderRepo) Update(ctx context.Context, conn *ProviderConnection) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE providers SET name=?, base_url=?, priority=?, weight=?, status=?, last_latency_ms=?, encrypted_secret=?, cooldown_until=?, last_error=?, last_error_at=?, backoff_level=?, updated_at=?
		 WHERE id = ?`,
		conn.Name, conn.BaseURL, conn.Priority, conn.Weight, conn.Status, conn.LastLatencyMs,
		conn.EncryptedSecret, conn.CooldownUntil, conn.LastError, conn.LastErrorAt,
		conn.BackoffLevel, conn.UpdatedAt, conn.ID)
	return err
}

func (r *sqliteProviderRepo) Delete(ctx context.Context, id string) error {
	_, err := r.driver.db.ExecContext(ctx, `DELETE FROM providers WHERE id = ?`, id)
	return err
}

func (r *sqliteProviderRepo) MarkCooldown(ctx context.Context, id string, until int64, errorMsg string) error {
	t := time.Unix(until, 0)
	now := time.Now()
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE providers SET status='cooldown', cooldown_until=?, last_error=?, last_error_at=?, updated_at=? WHERE id = ?`,
		t, errorMsg, now, now, id)
	return err
}

func (r *sqliteProviderRepo) ClearCooldown(ctx context.Context, id string) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE providers SET status='active', cooldown_until=NULL, last_error='', last_error_at=NULL, backoff_level=0, updated_at=? WHERE id = ?`,
		time.Now(), id)
	return err
}

func (r *sqliteProviderRepo) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE providers SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id)
	return err
}

func (r *sqliteProviderRepo) scanProviders(rows *sql.Rows) ([]*ProviderConnection, error) {
	var result []*ProviderConnection
	for rows.Next() {
		conn := &ProviderConnection{}
		err := rows.Scan(&conn.ID, &conn.Provider, &conn.Name, &conn.AuthType, &conn.EncryptedSecret,
			&conn.BaseURL, &conn.Priority, &conn.Weight, &conn.Status, &conn.LastLatencyMs,
			&conn.CooldownUntil, &conn.LastError, &conn.LastErrorAt, &conn.BackoffLevel,
			&conn.CreatedBy, &conn.CreatedAt, &conn.UpdatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, conn)
	}
	return result, nil
}

func (r *sqliteModelRepo) Create(ctx context.Context, m *Model) error {
	caps, _ := json.Marshal(m.Capabilities)
	tags, _ := json.Marshal(m.Tags)
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO models (id, provider_id, model_id, display_name, provider, model_name, mode, capabilities, context_window, max_output_tokens, input_price, output_price, input_cost_per_1k, output_cost_per_1k, enabled, is_active, tags, default_timeout_ms, supports_stream, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ProviderID, m.ModelID, m.DisplayName, m.Provider, m.ModelName, m.Mode,
		string(caps), m.ContextWindow, m.MaxOutputTokens, m.InputPrice, m.OutputPrice,
		m.InputCostPer1k, m.OutputCostPer1k, m.Enabled, m.IsActive, string(tags),
		m.DefaultTimeoutMs, m.SupportsStream, m.CreatedAt, m.UpdatedAt)
	return err
}

func (r *sqliteModelRepo) FindByID(ctx context.Context, id string) (*Model, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, provider_id, model_id, display_name, provider, model_name, mode, capabilities, context_window, max_output_tokens, input_price, output_price, input_cost_per_1k, output_cost_per_1k, enabled, is_active, tags, default_timeout_ms, supports_stream, created_at, updated_at
		 FROM models WHERE id = ?`, id)
	return r.scanModel(row)
}

func (r *sqliteModelRepo) List(ctx context.Context) ([]*Model, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, provider_id, model_id, display_name, provider, model_name, mode, capabilities, context_window, max_output_tokens, input_price, output_price, input_cost_per_1k, output_cost_per_1k, enabled, is_active, tags, default_timeout_ms, supports_stream, created_at, updated_at
		 FROM models ORDER BY display_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanModels(rows)
}

func (r *sqliteModelRepo) ListEnabled(ctx context.Context) ([]*Model, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, provider_id, model_id, display_name, provider, model_name, mode, capabilities, context_window, max_output_tokens, input_price, output_price, input_cost_per_1k, output_cost_per_1k, enabled, is_active, tags, default_timeout_ms, supports_stream, created_at, updated_at
		 FROM models WHERE enabled = 1 ORDER BY display_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanModels(rows)
}

func (r *sqliteModelRepo) ListByProvider(ctx context.Context, providerID string) ([]*Model, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, provider_id, model_id, display_name, provider, model_name, mode, capabilities, context_window, max_output_tokens, input_price, output_price, input_cost_per_1k, output_cost_per_1k, enabled, is_active, tags, default_timeout_ms, supports_stream, created_at, updated_at
		 FROM models WHERE provider_id = ? OR provider = ? ORDER BY display_name`,
		providerID, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanModels(rows)
}

func (r *sqliteModelRepo) Update(ctx context.Context, m *Model) error {
	caps, _ := json.Marshal(m.Capabilities)
	tags, _ := json.Marshal(m.Tags)
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE models SET provider_id=?, model_id=?, display_name=?, provider=?, model_name=?, mode=?, capabilities=?, context_window=?, max_output_tokens=?, input_price=?, output_price=?, input_cost_per_1k=?, output_cost_per_1k=?, enabled=?, is_active=?, tags=?, default_timeout_ms=?, supports_stream=?, updated_at=?
		 WHERE id = ?`,
		m.ProviderID, m.ModelID, m.DisplayName, m.Provider, m.ModelName, m.Mode,
		string(caps), m.ContextWindow, m.MaxOutputTokens, m.InputPrice, m.OutputPrice,
		m.InputCostPer1k, m.OutputCostPer1k, m.Enabled, m.IsActive, string(tags),
		m.DefaultTimeoutMs, m.SupportsStream, m.UpdatedAt, m.ID)
	return err
}

func (r *sqliteModelRepo) Delete(ctx context.Context, id string) error {
	_, err := r.driver.db.ExecContext(ctx, `DELETE FROM models WHERE id = ?`, id)
	return err
}

func (r *sqliteModelRepo) scanModel(row *sql.Row) (*Model, error) {
	m := &Model{}
	var caps, tags string
	err := row.Scan(&m.ID, &m.ProviderID, &m.ModelID, &m.DisplayName, &m.Provider, &m.ModelName,
		&m.Mode, &caps, &m.ContextWindow, &m.MaxOutputTokens, &m.InputPrice, &m.OutputPrice,
		&m.InputCostPer1k, &m.OutputCostPer1k, &m.Enabled, &m.IsActive, &tags,
		&m.DefaultTimeoutMs, &m.SupportsStream, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(caps), &m.Capabilities)
	json.Unmarshal([]byte(tags), &m.Tags)
	return m, nil
}

func (r *sqliteModelRepo) scanModels(rows *sql.Rows) ([]*Model, error) {
	var result []*Model
	for rows.Next() {
		m := &Model{}
		var caps, tags string
		err := rows.Scan(&m.ID, &m.ProviderID, &m.ModelID, &m.DisplayName, &m.Provider, &m.ModelName,
			&m.Mode, &caps, &m.ContextWindow, &m.MaxOutputTokens, &m.InputPrice, &m.OutputPrice,
			&m.InputCostPer1k, &m.OutputCostPer1k, &m.Enabled, &m.IsActive, &tags,
			&m.DefaultTimeoutMs, &m.SupportsStream, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(caps), &m.Capabilities)
		json.Unmarshal([]byte(tags), &m.Tags)
		result = append(result, m)
	}
	return result, nil
}

func (r *sqliteComboRepo) Create(ctx context.Context, c *Combo) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO combos (id, name, description, user_id, strategy, is_active, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Description, c.UserID, c.Strategy, c.IsActive, c.CreatedBy, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return err
	}
	for _, item := range c.Items {
		if err := r.AddItem(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (r *sqliteComboRepo) FindByID(ctx context.Context, id string) (*Combo, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, name, description, user_id, strategy, is_active, created_by, created_at, updated_at
		 FROM combos WHERE id = ?`, id)
	c := &Combo{}
	err := row.Scan(&c.ID, &c.Name, &c.Description, &c.UserID, &c.Strategy, &c.IsActive, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.Items, err = r.loadItems(ctx, c.ID)
	return c, err
}

func (r *sqliteComboRepo) FindByName(ctx context.Context, name string) (*Combo, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, name, description, user_id, strategy, is_active, created_by, created_at, updated_at
		 FROM combos WHERE name = ?`, name)
	c := &Combo{}
	err := row.Scan(&c.ID, &c.Name, &c.Description, &c.UserID, &c.Strategy, &c.IsActive, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.Items, err = r.loadItems(ctx, c.ID)
	return c, err
}

func (r *sqliteComboRepo) FindByUser(ctx context.Context, userID string) ([]*Combo, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, name, description, user_id, strategy, is_active, created_by, created_at, updated_at
		 FROM combos WHERE user_id = ? ORDER BY name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanCombos(ctx, rows)
}

func (r *sqliteComboRepo) List(ctx context.Context) ([]*Combo, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, name, description, user_id, strategy, is_active, created_by, created_at, updated_at
		 FROM combos ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanCombos(ctx, rows)
}

func (r *sqliteComboRepo) ListEnabled(ctx context.Context) ([]*Combo, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, name, description, user_id, strategy, is_active, created_by, created_at, updated_at
		 FROM combos WHERE is_active = 1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanCombos(ctx, rows)
}

func (r *sqliteComboRepo) Update(ctx context.Context, c *Combo) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE combos SET name=?, description=?, is_active=?, updated_at=? WHERE id = ?`,
		c.Name, c.Description, c.IsActive, c.UpdatedAt, c.ID)
	return err
}

func (r *sqliteComboRepo) Delete(ctx context.Context, id string) error {
	_, err := r.driver.db.ExecContext(ctx, `DELETE FROM combo_items WHERE combo_id = ?`, id)
	if err != nil {
		return err
	}
	_, err = r.driver.db.ExecContext(ctx, `DELETE FROM combos WHERE id = ?`, id)
	return err
}

func (r *sqliteComboRepo) AddItem(ctx context.Context, item *ComboItem) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO combo_items (id, combo_id, provider_id, model_id, priority, weight, max_retries, timeout_seconds, conditions)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.ComboID, item.ProviderID, item.ModelID, item.Priority,
		item.Weight, item.MaxRetries, item.TimeoutSeconds, item.Conditions)
	return err
}

func (r *sqliteComboRepo) RemoveItem(ctx context.Context, itemID string) error {
	_, err := r.driver.db.ExecContext(ctx, `DELETE FROM combo_items WHERE id = ?`, itemID)
	return err
}

func (r *sqliteComboRepo) ReorderItems(ctx context.Context, comboID string, itemIDs []string) error {
	for i, id := range itemIDs {
		if _, err := r.driver.db.ExecContext(ctx,
			`UPDATE combo_items SET priority = ? WHERE id = ? AND combo_id = ?`,
			i+1, id, comboID); err != nil {
			return err
		}
	}
	return nil
}

func (r *sqliteComboRepo) loadItems(ctx context.Context, comboID string) ([]*ComboItem, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, combo_id, provider_id, model_id, priority, weight, max_retries, timeout_seconds, conditions
		 FROM combo_items WHERE combo_id = ? ORDER BY priority`, comboID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*ComboItem
	for rows.Next() {
		item := &ComboItem{}
		err := rows.Scan(&item.ID, &item.ComboID, &item.ProviderID, &item.ModelID,
			&item.Priority, &item.Weight, &item.MaxRetries, &item.TimeoutSeconds, &item.Conditions)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *sqliteComboRepo) scanCombos(ctx context.Context, rows *sql.Rows) ([]*Combo, error) {
	var result []*Combo
	for rows.Next() {
		c := &Combo{}
		err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.UserID, &c.Strategy, &c.IsActive, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, err
		}
		c.Items, _ = r.loadItems(ctx, c.ID)
		result = append(result, c)
	}
	return result, nil
}

func (r *sqliteUsageRepo) Create(ctx context.Context, e *UsageEvent) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO usage_events (id, request_id, user_id, api_key_id, requested_model, final_model, provider, status_code, error_code, prompt_tokens, completion_tokens, total_tokens, estimated_cost, latency_ms, fallback_count, is_stream, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.RequestID, e.UserID, e.ApiKeyID, e.RequestedModel, e.FinalModel,
		e.Provider, e.StatusCode, e.ErrorCode, e.PromptTokens, e.CompletionTokens,
		e.TotalTokens, e.EstimatedCost, e.LatencyMs, e.FallbackCount, e.IsStream, e.CreatedAt)
	return err
}

func (r *sqliteUsageRepo) FindByID(ctx context.Context, id string) (*UsageEvent, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, request_id, user_id, api_key_id, requested_model, final_model, provider, status_code, error_code, prompt_tokens, completion_tokens, total_tokens, estimated_cost, latency_ms, fallback_count, is_stream, created_at
		 FROM usage_events WHERE id = ?`, id)
	e := &UsageEvent{}
	err := row.Scan(&e.ID, &e.RequestID, &e.UserID, &e.ApiKeyID, &e.RequestedModel, &e.FinalModel,
		&e.Provider, &e.StatusCode, &e.ErrorCode, &e.PromptTokens, &e.CompletionTokens,
		&e.TotalTokens, &e.EstimatedCost, &e.LatencyMs, &e.FallbackCount, &e.IsStream, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}

func (r *sqliteUsageRepo) FindByUserID(ctx context.Context, userID string, limit int) ([]*UsageEvent, error) {
	query := `SELECT id, request_id, user_id, api_key_id, requested_model, final_model, provider, status_code, error_code, prompt_tokens, completion_tokens, total_tokens, estimated_cost, latency_ms, fallback_count, is_stream, created_at
			  FROM usage_events WHERE user_id = ? ORDER BY created_at DESC`
	args := []interface{}{userID}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := r.driver.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanUsageEvents(rows)
}

func (r *sqliteUsageRepo) FindByDateRange(ctx context.Context, userID string, from, to int64) ([]*UsageEvent, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, request_id, user_id, api_key_id, requested_model, final_model, provider, status_code, error_code, prompt_tokens, completion_tokens, total_tokens, estimated_cost, latency_ms, fallback_count, is_stream, created_at
		 FROM usage_events WHERE user_id = ? AND created_at >= ? AND created_at <= ? ORDER BY created_at DESC`,
		userID, time.Unix(from, 0), time.Unix(to, 0))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanUsageEvents(rows)
}

func (r *sqliteUsageRepo) GetSummary(ctx context.Context, userID string, from, to int64) (*UsageSummary, error) {
	s := &UsageSummary{}
	err := r.driver.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0), COALESCE(SUM(total_tokens),0), COALESCE(SUM(estimated_cost),0), COALESCE(SUM(latency_ms),0),
		 SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END)
		 FROM usage_events WHERE user_id = ? AND created_at >= ? AND created_at <= ?`,
		userID, time.Unix(from, 0), time.Unix(to, 0)).Scan(
		&s.TotalRequests, &s.TotalPromptTokens, &s.TotalCompletionTokens,
		&s.TotalTokens, &s.TotalCost, &s.TotalLatencyMs, &s.ErrorCount)
	if s.TotalRequests > 0 {
		s.AvgLatencyMs = float64(s.TotalLatencyMs) / float64(s.TotalRequests)
	}
	return s, err
}

func (r *sqliteUsageRepo) Cleanup(ctx context.Context, before int64) (int64, error) {
	res, err := r.driver.db.ExecContext(ctx,
		`DELETE FROM usage_events WHERE created_at < ?`, time.Unix(before, 0))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *sqliteUsageRepo) scanUsageEvents(rows *sql.Rows) ([]*UsageEvent, error) {
	var result []*UsageEvent
	for rows.Next() {
		e := &UsageEvent{}
		err := rows.Scan(&e.ID, &e.RequestID, &e.UserID, &e.ApiKeyID, &e.RequestedModel, &e.FinalModel,
			&e.Provider, &e.StatusCode, &e.ErrorCode, &e.PromptTokens, &e.CompletionTokens,
			&e.TotalTokens, &e.EstimatedCost, &e.LatencyMs, &e.FallbackCount, &e.IsStream, &e.CreatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, nil
}

func (r *sqliteAuditRepo) Create(ctx context.Context, l *AuditLog) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO audit_logs (id, actor_id, actor_email, actor_role, action, target_type, target_id, details, ip_address, user_agent, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.ActorID, l.ActorEmail, l.ActorRole, l.Action,
		l.TargetType, l.TargetID, l.Details, l.IPAddress, l.UserAgent, l.CreatedAt)
	return err
}

func (r *sqliteAuditRepo) FindByID(ctx context.Context, id string) (*AuditLog, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, actor_id, actor_email, actor_role, action, target_type, target_id, details, ip_address, user_agent, created_at
		 FROM audit_logs WHERE id = ?`, id)
	l := &AuditLog{}
	err := row.Scan(&l.ID, &l.ActorID, &l.ActorEmail, &l.ActorRole, &l.Action,
		&l.TargetType, &l.TargetID, &l.Details, &l.IPAddress, &l.UserAgent, &l.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return l, err
}

func (r *sqliteAuditRepo) List(ctx context.Context, filter *AuditFilter) ([]*AuditLog, int64, error) {
	query := `SELECT id, actor_id, actor_email, actor_role, action, target_type, target_id, details, ip_address, user_agent, created_at
			  FROM audit_logs WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM audit_logs WHERE 1=1`
	args := []interface{}{}

	if filter.ActorID != "" {
		query += " AND actor_id = ?"
		countQuery += " AND actor_id = ?"
		args = append(args, filter.ActorID)
	}
	if filter.Action != "" {
		query += " AND action = ?"
		countQuery += " AND action = ?"
		args = append(args, filter.Action)
	}
	if filter.From > 0 {
		query += " AND created_at >= ?"
		countQuery += " AND created_at >= ?"
		args = append(args, time.Unix(filter.From, 0))
	}
	if filter.To > 0 {
		query += " AND created_at <= ?"
		countQuery += " AND created_at <= ?"
		args = append(args, time.Unix(filter.To, 0))
	}

	var total int64
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	r.driver.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)

	query += " ORDER BY created_at DESC"
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.driver.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []*AuditLog
	for rows.Next() {
		l := &AuditLog{}
		err := rows.Scan(&l.ID, &l.ActorID, &l.ActorEmail, &l.ActorRole, &l.Action,
			&l.TargetType, &l.TargetID, &l.Details, &l.IPAddress, &l.UserAgent, &l.CreatedAt)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, l)
	}
	return result, total, nil
}

func (r *sqliteSettingsRepo) Get(ctx context.Context) (*Settings, error) {
	s := &Settings{}
	err := r.driver.db.QueryRowContext(ctx,
		`SELECT require_login, require_api_key, enable_request_body_log, usage_retention_days, request_log_retention_days, audit_log_retention_days
		 FROM settings WHERE id = 1`).Scan(
		&s.RequireLogin, &s.RequireAPIKey, &s.EnableRequestBodyLog,
		&s.UsageRetentionDays, &s.RequestLogRetentionDays, &s.AuditLogRetentionDays)
	if err == sql.ErrNoRows {
		return s, nil
	}
	return s, err
}

func (r *sqliteSettingsRepo) Update(ctx context.Context, s *Settings) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE settings SET require_login=?, require_api_key=?, enable_request_body_log=?, usage_retention_days=?, request_log_retention_days=?, audit_log_retention_days=?
		 WHERE id = 1`,
		s.RequireLogin, s.RequireAPIKey, s.EnableRequestBodyLog,
		s.UsageRetentionDays, s.RequestLogRetentionDays, s.AuditLogRetentionDays)
	return err
}

func (r *sqliteRateLimitRepo) Create(ctx context.Context, rl *RateLimit) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO rate_limits (id, user_id, requests_per_minute, requests_per_day, tokens_per_minute, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rl.ID, rl.UserID, rl.RequestsPerMinute, rl.RequestsPerDay, rl.TokensPerMinute,
		rl.CreatedAt, rl.UpdatedAt)
	return err
}

func (r *sqliteRateLimitRepo) FindByID(ctx context.Context, id string) (*RateLimit, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, user_id, requests_per_minute, requests_per_day, tokens_per_minute, created_at, updated_at
		 FROM rate_limits WHERE id = ?`, id)
	rl := &RateLimit{}
	err := row.Scan(&rl.ID, &rl.UserID, &rl.RequestsPerMinute, &rl.RequestsPerDay,
		&rl.TokensPerMinute, &rl.CreatedAt, &rl.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return rl, err
}

func (r *sqliteRateLimitRepo) FindByUserID(ctx context.Context, userID string) (*RateLimit, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, user_id, requests_per_minute, requests_per_day, tokens_per_minute, created_at, updated_at
		 FROM rate_limits WHERE user_id = ?`, userID)
	rl := &RateLimit{}
	err := row.Scan(&rl.ID, &rl.UserID, &rl.RequestsPerMinute, &rl.RequestsPerDay,
		&rl.TokensPerMinute, &rl.CreatedAt, &rl.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return rl, err
}

func (r *sqliteRateLimitRepo) List(ctx context.Context) ([]*RateLimit, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, user_id, requests_per_minute, requests_per_day, tokens_per_minute, created_at, updated_at
		 FROM rate_limits ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*RateLimit
	for rows.Next() {
		rl := &RateLimit{}
		err := rows.Scan(&rl.ID, &rl.UserID, &rl.RequestsPerMinute, &rl.RequestsPerDay,
			&rl.TokensPerMinute, &rl.CreatedAt, &rl.UpdatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, rl)
	}
	return result, nil
}

func (r *sqliteRateLimitRepo) Update(ctx context.Context, rl *RateLimit) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE rate_limits SET requests_per_minute=?, requests_per_day=?, tokens_per_minute=?, updated_at=?
		 WHERE id = ?`,
		rl.RequestsPerMinute, rl.RequestsPerDay, rl.TokensPerMinute, rl.UpdatedAt, rl.ID)
	return err
}

func (r *sqliteRateLimitRepo) Delete(ctx context.Context, id string) error {
	_, err := r.driver.db.ExecContext(ctx, `DELETE FROM rate_limits WHERE id = ?`, id)
	return err
}

func (r *sqliteQuotaRepo) Create(ctx context.Context, q *Quota) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO quotas (id, user_id, monthly_token_cap, monthly_cost_cap, used_tokens, used_cost, reset_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		q.ID, q.UserID, q.MonthlyTokenCap, q.MonthlyCostCap, q.UsedTokens, q.UsedCost,
		q.ResetAt, q.CreatedAt, q.UpdatedAt)
	return err
}

func (r *sqliteQuotaRepo) FindByID(ctx context.Context, id string) (*Quota, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, user_id, monthly_token_cap, monthly_cost_cap, used_tokens, used_cost, reset_at, created_at, updated_at
		 FROM quotas WHERE id = ?`, id)
	q := &Quota{}
	err := row.Scan(&q.ID, &q.UserID, &q.MonthlyTokenCap, &q.MonthlyCostCap,
		&q.UsedTokens, &q.UsedCost, &q.ResetAt, &q.CreatedAt, &q.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return q, err
}

func (r *sqliteQuotaRepo) FindByUserID(ctx context.Context, userID string) (*Quota, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, user_id, monthly_token_cap, monthly_cost_cap, used_tokens, used_cost, reset_at, created_at, updated_at
		 FROM quotas WHERE user_id = ?`, userID)
	q := &Quota{}
	err := row.Scan(&q.ID, &q.UserID, &q.MonthlyTokenCap, &q.MonthlyCostCap,
		&q.UsedTokens, &q.UsedCost, &q.ResetAt, &q.CreatedAt, &q.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return q, err
}

func (r *sqliteQuotaRepo) List(ctx context.Context) ([]*Quota, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, user_id, monthly_token_cap, monthly_cost_cap, used_tokens, used_cost, reset_at, created_at, updated_at
		 FROM quotas ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Quota
	for rows.Next() {
		q := &Quota{}
		err := rows.Scan(&q.ID, &q.UserID, &q.MonthlyTokenCap, &q.MonthlyCostCap,
			&q.UsedTokens, &q.UsedCost, &q.ResetAt, &q.CreatedAt, &q.UpdatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, q)
	}
	return result, nil
}

func (r *sqliteQuotaRepo) Update(ctx context.Context, q *Quota) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE quotas SET monthly_token_cap=?, monthly_cost_cap=?, used_tokens=?, used_cost=?, reset_at=?, updated_at=?
		 WHERE id = ?`,
		q.MonthlyTokenCap, q.MonthlyCostCap, q.UsedTokens, q.UsedCost, q.ResetAt, q.UpdatedAt, q.ID)
	return err
}

func (r *sqliteQuotaRepo) IncrementUsage(ctx context.Context, userID string, tokens int64, cost float64) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE quotas SET used_tokens = used_tokens + ?, used_cost = used_cost + ?, updated_at = ?
		 WHERE user_id = ?`,
		tokens, cost, time.Now(), userID)
	return err
}

func (r *sqliteQuotaRepo) ResetMonthly(ctx context.Context, userID string) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE quotas SET used_tokens = 0, used_cost = 0, reset_at = ?, updated_at = ?
		 WHERE user_id = ?`,
		time.Now().AddDate(0, 1, 0), time.Now(), userID)
	return err
}

func (r *sqliteQuotaRepo) Delete(ctx context.Context, id string) error {
	_, err := r.driver.db.ExecContext(ctx, `DELETE FROM quotas WHERE id = ?`, id)
	return err
}

func (r *sqliteAliasRepo) Create(ctx context.Context, a *ModelAlias) error {
	_, err := r.driver.db.ExecContext(ctx,
		`INSERT INTO model_aliases (id, name, target_id, provider, description, user_id, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Name, a.TargetID, a.Provider, a.Description, a.UserID, a.CreatedBy, a.CreatedAt)
	return err
}

func (r *sqliteAliasRepo) FindByID(ctx context.Context, id string) (*ModelAlias, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, name, target_id, provider, description, user_id, created_by, created_at
		 FROM model_aliases WHERE id = ?`, id)
	return r.scanAlias(row)
}

func (r *sqliteAliasRepo) FindByName(ctx context.Context, name string) (*ModelAlias, error) {
	row := r.driver.db.QueryRowContext(ctx,
		`SELECT id, name, target_id, provider, description, user_id, created_by, created_at
		 FROM model_aliases WHERE name = ?`, name)
	return r.scanAlias(row)
}

func (r *sqliteAliasRepo) FindByUser(ctx context.Context, userID string) ([]*ModelAlias, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, name, target_id, provider, description, user_id, created_by, created_at
		 FROM model_aliases WHERE user_id = ? ORDER BY name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanAliases(rows)
}

func (r *sqliteAliasRepo) ListGlobal(ctx context.Context) ([]*ModelAlias, error) {
	rows, err := r.driver.db.QueryContext(ctx,
		`SELECT id, name, target_id, provider, description, user_id, created_by, created_at
		 FROM model_aliases WHERE user_id = '' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanAliases(rows)
}

func (r *sqliteAliasRepo) Update(ctx context.Context, a *ModelAlias) error {
	_, err := r.driver.db.ExecContext(ctx,
		`UPDATE model_aliases SET name=?, target_id=?, provider=?, description=? WHERE id = ?`,
		a.Name, a.TargetID, a.Provider, a.Description, a.ID)
	return err
}

func (r *sqliteAliasRepo) Delete(ctx context.Context, id string) error {
	_, err := r.driver.db.ExecContext(ctx, `DELETE FROM model_aliases WHERE id = ?`, id)
	return err
}

func (r *sqliteAliasRepo) scanAlias(row *sql.Row) (*ModelAlias, error) {
	a := &ModelAlias{}
	err := row.Scan(&a.ID, &a.Name, &a.TargetID, &a.Provider, &a.Description, &a.UserID, &a.CreatedBy, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (r *sqliteAliasRepo) scanAliases(rows *sql.Rows) ([]*ModelAlias, error) {
	var result []*ModelAlias
	for rows.Next() {
		a := &ModelAlias{}
		if err := rows.Scan(&a.ID, &a.Name, &a.TargetID, &a.Provider, &a.Description, &a.UserID, &a.CreatedBy, &a.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, nil
}
