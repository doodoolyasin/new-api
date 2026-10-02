package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"ai-gateway/models"

	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrKeyNotFound      = errors.New("virtual key not found")
	ErrKeyRevoked       = errors.New("virtual key has been revoked")
	ErrKeyExpired       = errors.New("virtual key has expired")
	ErrKeyDisabled      = errors.New("virtual key is disabled or suspended")
	ErrInsufficientQuota = errors.New("insufficient quota")
	ErrMaxIPsExceeded   = errors.New("maximum allowed IPs exceeded for this key")
)

type DB struct {
	conn *sql.DB
	mu   sync.RWMutex
}

func HashKey(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

func InitDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(10 * time.Minute)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return db, nil
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS virtual_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key_id TEXT UNIQUE NOT NULL,
		key_hash TEXT UNIQUE NOT NULL,
		owner_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'ACTIVE',
		plan_id TEXT NOT NULL DEFAULT 'FREE',
		note TEXT DEFAULT '',
		total_quota INTEGER NOT NULL DEFAULT 0,
		remain_quota INTEGER NOT NULL DEFAULT 0,
		consumed_quota INTEGER NOT NULL DEFAULT 0,
		rpm_limit INTEGER NOT NULL DEFAULT 10,
		max_concurrency INTEGER NOT NULL DEFAULT 1,
		max_allowed_ips INTEGER NOT NULL DEFAULT 2,
		allowed_models TEXT NOT NULL DEFAULT '*',
		blocked_models TEXT NOT NULL DEFAULT '',
		current_ip_count INTEGER NOT NULL DEFAULT 0,
		last_ip TEXT DEFAULT '',
		last_used_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		expired_at DATETIME,
		revoked_at DATETIME,
		regeneration_history TEXT DEFAULT '[]'
	);

	CREATE INDEX IF NOT EXISTS idx_vk_hash ON virtual_keys(key_hash);
	CREATE INDEX IF NOT EXISTS idx_vk_owner ON virtual_keys(owner_id);
	CREATE INDEX IF NOT EXISTS idx_vk_status ON virtual_keys(status);

	CREATE TABLE IF NOT EXISTS providers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		base_url TEXT NOT NULL,
		api_key TEXT NOT NULL,
		supported_models TEXT NOT NULL DEFAULT '*',
		priority INTEGER NOT NULL DEFAULT 1,
		weight INTEGER NOT NULL DEFAULT 10,
		status TEXT NOT NULL DEFAULT 'ACTIVE',
		consecutive_failures INTEGER NOT NULL DEFAULT 0,
		cooldown_until DATETIME,
		last_success_at DATETIME,
		last_error_at DATETIME,
		last_error TEXT DEFAULT '',
		avg_latency_ms INTEGER DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS model_pricing (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		model_name TEXT UNIQUE NOT NULL,
		input_multiplier REAL NOT NULL DEFAULT 1.0,
		output_multiplier REAL NOT NULL DEFAULT 1.0,
		min_charge INTEGER NOT NULL DEFAULT 1,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS quota_transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key_id TEXT NOT NULL,
		type TEXT NOT NULL,
		amount INTEGER NOT NULL,
		balance INTEGER NOT NULL,
		model TEXT DEFAULT '',
		request_id TEXT DEFAULT '',
		details TEXT DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_tx_key ON quota_transactions(key_id);
	CREATE INDEX IF NOT EXISTS idx_tx_created ON quota_transactions(created_at);

	CREATE TABLE IF NOT EXISTS ip_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key_id TEXT NOT NULL,
		ip TEXT NOT NULL,
		first_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		hits INTEGER NOT NULL DEFAULT 1,
		UNIQUE(key_id, ip)
	);

	CREATE INDEX IF NOT EXISTS idx_ip_key ON ip_records(key_id);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		actor TEXT NOT NULL,
		target TEXT NOT NULL,
		action TEXT NOT NULL,
		metadata TEXT DEFAULT '',
		remote_addr TEXT DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS coupons (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT UNIQUE NOT NULL,
		quota INTEGER NOT NULL DEFAULT 0,
		days INTEGER NOT NULL DEFAULT 0,
		max_uses INTEGER NOT NULL DEFAULT 1,
		used_count INTEGER NOT NULL DEFAULT 0,
		plan_id TEXT DEFAULT 'FREE',
		expired_at DATETIME,
		is_active BOOLEAN NOT NULL DEFAULT 1,
		created_by TEXT DEFAULT 'admin',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS coupon_redemptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		coupon_code TEXT NOT NULL,
		user_id TEXT NOT NULL,
		key_id TEXT NOT NULL,
		redeemed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(coupon_code, user_id)
	);
	`

	_, err := d.conn.Exec(schema)
	if err != nil {
		return err
	}

	// Seed default model pricing if empty
	defaultPricings := []models.ModelPricing{
		{ModelName: "claude-opus-5.5", InputMultiplier: 5.0, OutputMultiplier: 5.0, MinCharge: 1, Enabled: true},
		{ModelName: "claude-sonnet-3.5", InputMultiplier: 3.0, OutputMultiplier: 3.0, MinCharge: 1, Enabled: true},
		{ModelName: "deepseek-chat", InputMultiplier: 1.0, OutputMultiplier: 1.0, MinCharge: 1, Enabled: true},
		{ModelName: "deepseek-coder", InputMultiplier: 1.0, OutputMultiplier: 1.0, MinCharge: 1, Enabled: true},
		{ModelName: "*", InputMultiplier: 1.0, OutputMultiplier: 1.0, MinCharge: 1, Enabled: true},
	}
	for _, p := range defaultPricings {
		_, _ = d.conn.Exec(`INSERT OR IGNORE INTO model_pricing(model_name, input_multiplier, output_multiplier, min_charge, enabled) VALUES (?, ?, ?, ?, ?)`,
			p.ModelName, p.InputMultiplier, p.OutputMultiplier, p.MinCharge, p.Enabled)
	}

	return nil
}

func (d *DB) GetVirtualKeyByHash(keyHash string) (*models.VirtualKey, error) {
	query := `SELECT id, key_id, key_hash, owner_id, status, plan_id, note, total_quota, remain_quota, consumed_quota,
		rpm_limit, max_concurrency, max_allowed_ips, allowed_models, blocked_models, current_ip_count, last_ip,
		last_used_at, created_at, updated_at, expired_at, revoked_at, regeneration_history
		FROM virtual_keys WHERE key_hash = ?`

	row := d.conn.QueryRow(query, keyHash)
	vk := &models.VirtualKey{}
	var lastUsedAt, expiredAt, revokedAt sql.NullTime

	err := row.Scan(
		&vk.ID, &vk.KeyID, &vk.KeyHash, &vk.OwnerID, &vk.Status, &vk.PlanID, &vk.Note,
		&vk.TotalQuota, &vk.RemainQuota, &vk.ConsumedQuota, &vk.RPMLimit, &vk.MaxConcurrency,
		&vk.MaxAllowedIPs, &vk.AllowedModels, &vk.BlockedModels, &vk.CurrentIPCount, &vk.LastIP,
		&lastUsedAt, &vk.CreatedAt, &vk.UpdatedAt, &expiredAt, &revokedAt, &vk.RegenerationHistory,
	)
	if err == sql.ErrNoRows {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}

	if lastUsedAt.Valid {
		vk.LastUsedAt = &lastUsedAt.Time
	}
	if expiredAt.Valid {
		vk.ExpiredAt = &expiredAt.Time
	}
	if revokedAt.Valid {
		vk.RevokedAt = &revokedAt.Time
	}

	return vk, nil
}

func (d *DB) GetVirtualKeyByID(keyID string) (*models.VirtualKey, error) {
	query := `SELECT id, key_id, key_hash, owner_id, status, plan_id, note, total_quota, remain_quota, consumed_quota,
		rpm_limit, max_concurrency, max_allowed_ips, allowed_models, blocked_models, current_ip_count, last_ip,
		last_used_at, created_at, updated_at, expired_at, revoked_at, regeneration_history
		FROM virtual_keys WHERE key_id = ?`

	row := d.conn.QueryRow(query, keyID)
	vk := &models.VirtualKey{}
	var lastUsedAt, expiredAt, revokedAt sql.NullTime

	err := row.Scan(
		&vk.ID, &vk.KeyID, &vk.KeyHash, &vk.OwnerID, &vk.Status, &vk.PlanID, &vk.Note,
		&vk.TotalQuota, &vk.RemainQuota, &vk.ConsumedQuota, &vk.RPMLimit, &vk.MaxConcurrency,
		&vk.MaxAllowedIPs, &vk.AllowedModels, &vk.BlockedModels, &vk.CurrentIPCount, &vk.LastIP,
		&lastUsedAt, &vk.CreatedAt, &vk.UpdatedAt, &expiredAt, &revokedAt, &vk.RegenerationHistory,
	)
	if err == sql.ErrNoRows {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}

	if lastUsedAt.Valid {
		vk.LastUsedAt = &lastUsedAt.Time
	}
	if expiredAt.Valid {
		vk.ExpiredAt = &expiredAt.Time
	}
	if revokedAt.Valid {
		vk.RevokedAt = &revokedAt.Time
	}

	return vk, nil
}

func (d *DB) GetVirtualKeysByOwner(ownerID string) ([]models.VirtualKey, error) {
	query := `SELECT id, key_id, key_hash, owner_id, status, plan_id, note, total_quota, remain_quota, consumed_quota,
		rpm_limit, max_concurrency, max_allowed_ips, allowed_models, blocked_models, current_ip_count, last_ip,
		last_used_at, created_at, updated_at, expired_at, revoked_at, regeneration_history
		FROM virtual_keys WHERE owner_id = ? ORDER BY id DESC`

	rows, err := d.conn.Query(query, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []models.VirtualKey
	for rows.Next() {
		vk := models.VirtualKey{}
		var lastUsedAt, expiredAt, revokedAt sql.NullTime

		if err := rows.Scan(
			&vk.ID, &vk.KeyID, &vk.KeyHash, &vk.OwnerID, &vk.Status, &vk.PlanID, &vk.Note,
			&vk.TotalQuota, &vk.RemainQuota, &vk.ConsumedQuota, &vk.RPMLimit, &vk.MaxConcurrency,
			&vk.MaxAllowedIPs, &vk.AllowedModels, &vk.BlockedModels, &vk.CurrentIPCount, &vk.LastIP,
			&lastUsedAt, &vk.CreatedAt, &vk.UpdatedAt, &expiredAt, &revokedAt, &vk.RegenerationHistory,
		); err != nil {
			return nil, err
		}

		if lastUsedAt.Valid {
			vk.LastUsedAt = &lastUsedAt.Time
		}
		if expiredAt.Valid {
			vk.ExpiredAt = &expiredAt.Time
		}
		if revokedAt.Valid {
			vk.RevokedAt = &revokedAt.Time
		}
		keys = append(keys, vk)
	}

	return keys, nil
}

func (d *DB) CreateVirtualKey(vk *models.VirtualKey) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT INTO virtual_keys (
		key_id, key_hash, owner_id, status, plan_id, note, total_quota, remain_quota, consumed_quota,
		rpm_limit, max_concurrency, max_allowed_ips, allowed_models, blocked_models, expired_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	res, err := tx.Exec(query,
		vk.KeyID, vk.KeyHash, vk.OwnerID, vk.Status, vk.PlanID, vk.Note,
		vk.TotalQuota, vk.RemainQuota, 0,
		vk.RPMLimit, vk.MaxConcurrency, vk.MaxAllowedIPs, vk.AllowedModels, vk.BlockedModels,
		vk.ExpiredAt,
	)
	if err != nil {
		return err
	}

	id, _ := res.LastInsertId()
	vk.ID = id

	// Initial quota ledger entry
	if vk.RemainQuota > 0 {
		_, err = tx.Exec(`INSERT INTO quota_transactions (key_id, type, amount, balance, details) VALUES (?, ?, ?, ?, ?)`,
			vk.KeyID, models.TxInitialAllocation, vk.RemainQuota, vk.RemainQuota, "Initial allocation upon key creation")
		if err != nil {
			return err
		}
	}

	// Audit log
	_, err = tx.Exec(`INSERT INTO audit_logs (actor, target, action, metadata) VALUES (?, ?, ?, ?)`,
		"system", vk.KeyID, "KEY_CREATED", fmt.Sprintf("owner:%s, plan:%s, quota:%d", vk.OwnerID, vk.PlanID, vk.RemainQuota))
	if err != nil {
		return err
	}

	return tx.Commit()
}

// RegenerateVirtualKey atomically revokes old key and creates new key with preserved attributes
func (d *DB) RegenerateVirtualKey(oldKeyID, newKeyID, newKeyHash, actor string) (*models.VirtualKey, error) {
	tx, err := d.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	oldKey, err := d.GetVirtualKeyByID(oldKeyID)
	if err != nil {
		return nil, err
	}
	if oldKey.Status == models.StatusRevoked {
		return nil, ErrKeyRevoked
	}

	// Revoke old key
	now := time.Now()
	_, err = tx.Exec(`UPDATE virtual_keys SET status = ?, revoked_at = ?, updated_at = ? WHERE key_id = ?`,
		models.StatusRevoked, now, now, oldKeyID)
	if err != nil {
		return nil, err
	}

	// Build regeneration history
	var history []string
	if oldKey.RegenerationHistory != "" {
		_ = json.Unmarshal([]byte(oldKey.RegenerationHistory), &history)
	}
	history = append(history, oldKeyID)
	historyJSON, _ := json.Marshal(history)

	// Insert new key
	query := `INSERT INTO virtual_keys (
		key_id, key_hash, owner_id, status, plan_id, note, total_quota, remain_quota, consumed_quota,
		rpm_limit, max_concurrency, max_allowed_ips, allowed_models, blocked_models, expired_at, regeneration_history
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	res, err := tx.Exec(query,
		newKeyID, newKeyHash, oldKey.OwnerID, models.StatusActive, oldKey.PlanID, oldKey.Note,
		oldKey.TotalQuota, oldKey.RemainQuota, oldKey.ConsumedQuota,
		oldKey.RPMLimit, oldKey.MaxConcurrency, oldKey.MaxAllowedIPs, oldKey.AllowedModels, oldKey.BlockedModels,
		oldKey.ExpiredAt, string(historyJSON),
	)
	if err != nil {
		return nil, err
	}

	newID, _ := res.LastInsertId()

	// Audit log
	_, err = tx.Exec(`INSERT INTO audit_logs (actor, target, action, metadata) VALUES (?, ?, ?, ?)`,
		actor, newKeyID, "KEY_REGENERATED", fmt.Sprintf("replaced_key:%s, owner:%s, preserved_quota:%d", oldKeyID, oldKey.OwnerID, oldKey.RemainQuota))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	newKey := *oldKey
	newKey.ID = newID
	newKey.KeyID = newKeyID
	newKey.KeyHash = newKeyHash
	newKey.Status = models.StatusActive
	newKey.RegenerationHistory = string(historyJSON)
	return &newKey, nil
}

func (d *DB) RevokeVirtualKey(keyID, actor string) error {
	now := time.Now()
	res, err := d.conn.Exec(`UPDATE virtual_keys SET status = ?, revoked_at = ?, updated_at = ? WHERE key_id = ? AND status != ?`,
		models.StatusRevoked, now, now, keyID, models.StatusRevoked)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrKeyNotFound
	}

	_, _ = d.conn.Exec(`INSERT INTO audit_logs (actor, target, action, metadata) VALUES (?, ?, ?, ?)`,
		actor, keyID, "KEY_REVOKED", "Admin or user revoked key")

	return nil
}

// DeductQuota atomically subtracts tokens, updates consumed tokens, and logs transaction
func (d *DB) DeductQuota(keyID string, tokensToDeduct int64, model string, reqID string) (int64, error) {
	tx, err := d.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Atomic update: only if remain_quota >= tokensToDeduct
	res, err := tx.Exec(`UPDATE virtual_keys 
		SET remain_quota = remain_quota - ?, consumed_quota = consumed_quota + ?, updated_at = CURRENT_TIMESTAMP
		WHERE key_id = ? AND remain_quota >= ?`,
		tokensToDeduct, tokensToDeduct, keyID, tokensToDeduct)
	if err != nil {
		return 0, err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		// Either insufficient quota or key not found
		var currentRemain int64
		err := tx.QueryRow(`SELECT remain_quota FROM virtual_keys WHERE key_id = ?`, keyID).Scan(&currentRemain)
		if err != nil {
			return 0, ErrKeyNotFound
		}
		return currentRemain, ErrInsufficientQuota
	}

	var newBalance int64
	err = tx.QueryRow(`SELECT remain_quota FROM virtual_keys WHERE key_id = ?`, keyID).Scan(&newBalance)
	if err != nil {
		return 0, err
	}

	// Update status to EXHAUSTED if remain_quota == 0
	if newBalance == 0 {
		_, _ = tx.Exec(`UPDATE virtual_keys SET status = ? WHERE key_id = ? AND status = ?`,
			models.StatusExhausted, keyID, models.StatusActive)
	}

	// Ledger transaction record
	_, err = tx.Exec(`INSERT INTO quota_transactions (key_id, type, amount, balance, model, request_id, details) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		keyID, models.TxUsage, -tokensToDeduct, newBalance, model, reqID, fmt.Sprintf("Deducted %d tokens for %s", tokensToDeduct, model))
	if err != nil {
		return 0, err
	}

	return newBalance, tx.Commit()
}

// AddQuota adds tokens to a virtual key and logs to ledger
func (d *DB) AddQuota(keyID string, tokensToAdd int64, txType string, details string, actor string) (int64, error) {
	tx, err := d.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`UPDATE virtual_keys 
		SET remain_quota = remain_quota + ?, total_quota = total_quota + ?, 
		    status = CASE WHEN status = 'EXHAUSTED' THEN 'ACTIVE' ELSE status END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE key_id = ?`,
		tokensToAdd, tokensToAdd, keyID)
	if err != nil {
		return 0, err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return 0, ErrKeyNotFound
	}

	var newBalance int64
	err = tx.QueryRow(`SELECT remain_quota FROM virtual_keys WHERE key_id = ?`, keyID).Scan(&newBalance)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(`INSERT INTO quota_transactions (key_id, type, amount, balance, details) VALUES (?, ?, ?, ?, ?)`,
		keyID, txType, tokensToAdd, newBalance, details)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(`INSERT INTO audit_logs (actor, target, action, metadata) VALUES (?, ?, ?, ?)`,
		actor, keyID, "QUOTA_ADDED", fmt.Sprintf("amount:%d, new_balance:%d", tokensToAdd, newBalance))
	if err != nil {
		return 0, err
	}

	return newBalance, tx.Commit()
}

// CheckAndRegisterIP validates and registers client IP with max_allowed_ips constraint
func (d *DB) CheckAndRegisterIP(keyID string, clientIP string, maxAllowedIPs int) (bool, error) {
	tx, err := d.conn.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// Check if this IP is already registered
	var hits int64
	err = tx.QueryRow(`SELECT hits FROM ip_records WHERE key_id = ? AND ip = ?`, keyID, clientIP).Scan(&hits)
	if err == nil {
		// Existing IP: update last_seen and hits
		_, _ = tx.Exec(`UPDATE ip_records SET hits = hits + 1, last_seen = CURRENT_TIMESTAMP WHERE key_id = ? AND ip = ?`, keyID, clientIP)
		_, _ = tx.Exec(`UPDATE virtual_keys SET last_ip = ?, last_used_at = CURRENT_TIMESTAMP WHERE key_id = ?`, clientIP, keyID)
		return true, tx.Commit()
	}

	if err != sql.ErrNoRows {
		return false, err
	}

	// New IP: count distinct IPs for this key
	var count int
	err = tx.QueryRow(`SELECT COUNT(*) FROM ip_records WHERE key_id = ?`, keyID).Scan(&count)
	if err != nil {
		return false, err
	}

	if count >= maxAllowedIPs {
		return false, ErrMaxIPsExceeded
	}

	// Insert new IP record
	_, err = tx.Exec(`INSERT INTO ip_records (key_id, ip) VALUES (?, ?)`, keyID, clientIP)
	if err != nil {
		return false, err
	}

	// Update virtual_keys
	_, err = tx.Exec(`UPDATE virtual_keys SET current_ip_count = ?, last_ip = ?, last_used_at = CURRENT_TIMESTAMP WHERE key_id = ?`,
		count+1, clientIP, keyID)
	if err != nil {
		return false, err
	}

	return true, tx.Commit()
}

// GetModelPricing retrieves pricing multiplier for a model
func (d *DB) GetModelPricing(modelName string) (*models.ModelPricing, error) {
	row := d.conn.QueryRow(`SELECT id, model_name, input_multiplier, output_multiplier, min_charge, enabled 
		FROM model_pricing WHERE model_name = ? AND enabled = 1`, modelName)
	p := &models.ModelPricing{}
	err := row.Scan(&p.ID, &p.ModelName, &p.InputMultiplier, &p.OutputMultiplier, &p.MinCharge, &p.Enabled)
	if err == nil {
		return p, nil
	}

	// Fallback to wildcard "*"
	row = d.conn.QueryRow(`SELECT id, model_name, input_multiplier, output_multiplier, min_charge, enabled 
		FROM model_pricing WHERE model_name = '*' AND enabled = 1`)
	err = row.Scan(&p.ID, &p.ModelName, &p.InputMultiplier, &p.OutputMultiplier, &p.MinCharge, &p.Enabled)
	if err == nil {
		return p, nil
	}

	// Hard default
	return &models.ModelPricing{
		ModelName:        modelName,
		InputMultiplier:  1.0,
		OutputMultiplier: 1.0,
		MinCharge:        1,
		Enabled:          true,
	}, nil
}

// Providers CRUD & State
func (d *DB) GetActiveProviders(model string) ([]models.Provider, error) {
	rows, err := d.conn.Query(`SELECT id, name, base_url, api_key, supported_models, priority, weight, status, 
		consecutive_failures, cooldown_until, last_success_at, last_error_at, last_error, avg_latency_ms
		FROM providers WHERE status != 'DISABLED' ORDER BY priority ASC, weight DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []models.Provider
	now := time.Now()

	for rows.Next() {
		p := models.Provider{}
		var cooldownUntil, lastSuccess, lastError sql.NullTime

		if err := rows.Scan(
			&p.ID, &p.Name, &p.BaseURL, &p.APIKey, &p.SupportedModels, &p.Priority, &p.Weight, &p.Status,
			&p.ConsecutiveFailures, &cooldownUntil, &lastSuccess, &lastError, &p.LastError, &p.AvgLatencyMs,
		); err != nil {
			return nil, err
		}

		if cooldownUntil.Valid {
			p.CooldownUntil = &cooldownUntil.Time
			// If cooldown expired, treat as active
			if now.After(*p.CooldownUntil) {
				p.Status = models.StatusActive
			}
		}
		if lastSuccess.Valid {
			p.LastSuccessAt = &lastSuccess.Time
		}
		if lastError.Valid {
			p.LastErrorAt = &lastError.Time
		}

		// Filter for supported models
		if p.SupportedModels == "*" || p.SupportedModels == "" || containsModel(p.SupportedModels, model) {
			providers = append(providers, p)
		}
	}

	return providers, nil
}

func (d *DB) UpdateProviderSuccess(id int64, latencyMs int64) {
	now := time.Now()
	_, _ = d.conn.Exec(`UPDATE providers 
		SET status = 'ACTIVE', consecutive_failures = 0, cooldown_until = NULL, 
		    last_success_at = ?, avg_latency_ms = (avg_latency_ms + ?) / 2, updated_at = ?
		WHERE id = ?`, now, latencyMs, now, id)
}

func (d *DB) UpdateProviderFailure(id int64, errMsg string, cooldownSec int) {
	now := time.Now()
	cooldown := now.Add(time.Duration(cooldownSec) * time.Second)
	_, _ = d.conn.Exec(`UPDATE providers 
		SET consecutive_failures = consecutive_failures + 1, status = 'COOLDOWN', cooldown_until = ?, 
		    last_error_at = ?, last_error = ?, updated_at = ?
		WHERE id = ?`, cooldown, now, errMsg, now, id)
}

func (d *DB) InsertProvider(p *models.Provider) error {
	res, err := d.conn.Exec(`INSERT INTO providers (name, base_url, api_key, supported_models, priority, weight, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.BaseURL, p.APIKey, p.SupportedModels, p.Priority, p.Weight, p.Status)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	p.ID = id
	return nil
}

func (d *DB) GetAllProviders() ([]models.Provider, error) {
	rows, err := d.conn.Query(`SELECT id, name, base_url, api_key, supported_models, priority, weight, status, 
		consecutive_failures, cooldown_until, last_success_at, last_error_at, last_error, avg_latency_ms
		FROM providers ORDER BY priority ASC, weight DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []models.Provider
	for rows.Next() {
		p := models.Provider{}
		var cooldownUntil, lastSuccess, lastError sql.NullTime
		if err := rows.Scan(
			&p.ID, &p.Name, &p.BaseURL, &p.APIKey, &p.SupportedModels, &p.Priority, &p.Weight, &p.Status,
			&p.ConsecutiveFailures, &cooldownUntil, &lastSuccess, &lastError, &p.LastError, &p.AvgLatencyMs,
		); err != nil {
			return nil, err
		}
		if cooldownUntil.Valid {
			p.CooldownUntil = &cooldownUntil.Time
		}
		if lastSuccess.Valid {
			p.LastSuccessAt = &lastSuccess.Time
		}
		if lastError.Valid {
			p.LastErrorAt = &lastError.Time
		}
		providers = append(providers, p)
	}
	return providers, nil
}

// Coupons / Redeem Codes
func (d *DB) RedeemCoupon(code, userID, keyID, actor string) (int64, error) {
	tx, err := d.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// 1. Check coupon validity
	var couponID, quota, maxUses, usedCount int64
	var isActive bool
	var expiredAt sql.NullTime
	err = tx.QueryRow(`SELECT id, quota, max_uses, used_count, is_active, expired_at FROM coupons WHERE code = ?`, code).
		Scan(&couponID, &quota, &maxUses, &usedCount, &isActive, &expiredAt)
	if err == sql.ErrNoRows {
		return 0, errors.New("coupon not found")
	}
	if err != nil {
		return 0, err
	}
	if !isActive {
		return 0, errors.New("coupon is disabled")
	}
	if maxUses > 0 && usedCount >= maxUses {
		return 0, errors.New("coupon usage limit reached")
	}
	if expiredAt.Valid && time.Now().After(expiredAt.Time) {
		return 0, errors.New("coupon has expired")
	}

	// 2. Check if user already redeemed this coupon
	var exists int
	err = tx.QueryRow(`SELECT 1 FROM coupon_redemptions WHERE coupon_code = ? AND user_id = ?`, code, userID).Scan(&exists)
	if err == nil {
		return 0, errors.New("you have already redeemed this coupon")
	}

	// 3. Mark redemption
	_, err = tx.Exec(`INSERT INTO coupon_redemptions (coupon_code, user_id, key_id) VALUES (?, ?, ?)`, code, userID, keyID)
	if err != nil {
		return 0, err
	}

	// 4. Increment coupon usage
	_, err = tx.Exec(`UPDATE coupons SET used_count = used_count + 1 WHERE id = ?`, couponID)
	if err != nil {
		return 0, err
	}

	// 5. Add quota to the key
	_, err = tx.Exec(`UPDATE virtual_keys SET remain_quota = remain_quota + ?, total_quota = total_quota + ?, updated_at = CURRENT_TIMESTAMP WHERE key_id = ?`,
		quota, quota, keyID)
	if err != nil {
		return 0, err
	}

	var newBalance int64
	_ = tx.QueryRow(`SELECT remain_quota FROM virtual_keys WHERE key_id = ?`, keyID).Scan(&newBalance)

	// 6. Ledger transaction
	_, _ = tx.Exec(`INSERT INTO quota_transactions (key_id, type, amount, balance, details) VALUES (?, ?, ?, ?, ?)`,
		keyID, models.TxCouponReward, quota, newBalance, fmt.Sprintf("Redeemed coupon %s", code))

	// 7. Audit log
	_, _ = tx.Exec(`INSERT INTO audit_logs (actor, target, action, metadata) VALUES (?, ?, ?, ?)`,
		actor, keyID, "COUPON_REDEEMED", fmt.Sprintf("code:%s, user:%s, quota:%d", code, userID, quota))

	return quota, tx.Commit()
}

func containsModel(list, model string) bool {
	if list == "*" {
		return true
	}
	for _, m := range splitComma(list) {
		if m == model || m == "*" {
			return true
		}
	}
	return false
}

func splitComma(s string) []string {
	var res []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			item := s[start:i]
			if item != "" {
				res = append(res, item)
			}
			start = i + 1
		}
	}
	return res
}

func (d *DB) GetAllVirtualKeys(limit, offset int) ([]models.VirtualKey, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `SELECT id, key_id, key_hash, owner_id, status, plan_id, note, total_quota, remain_quota, consumed_quota,
		rpm_limit, max_concurrency, max_allowed_ips, allowed_models, blocked_models, current_ip_count, last_ip,
		last_used_at, created_at, updated_at, expired_at, revoked_at, regeneration_history
		FROM virtual_keys ORDER BY id DESC LIMIT ? OFFSET ?`

	rows, err := d.conn.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []models.VirtualKey
	for rows.Next() {
		vk := models.VirtualKey{}
		var lastUsedAt, expiredAt, revokedAt sql.NullTime
		if err := rows.Scan(
			&vk.ID, &vk.KeyID, &vk.KeyHash, &vk.OwnerID, &vk.Status, &vk.PlanID, &vk.Note,
			&vk.TotalQuota, &vk.RemainQuota, &vk.ConsumedQuota, &vk.RPMLimit, &vk.MaxConcurrency,
			&vk.MaxAllowedIPs, &vk.AllowedModels, &vk.BlockedModels, &vk.CurrentIPCount, &vk.LastIP,
			&lastUsedAt, &vk.CreatedAt, &vk.UpdatedAt, &expiredAt, &revokedAt, &vk.RegenerationHistory,
		); err != nil {
			return nil, err
		}
		if lastUsedAt.Valid {
			vk.LastUsedAt = &lastUsedAt.Time
		}
		if expiredAt.Valid {
			vk.ExpiredAt = &expiredAt.Time
		}
		if revokedAt.Valid {
			vk.RevokedAt = &revokedAt.Time
		}
		keys = append(keys, vk)
	}
	return keys, nil
}

func (d *DB) GetAllModelPricing() ([]models.ModelPricing, error) {
	rows, err := d.conn.Query(`SELECT id, model_name, input_multiplier, output_multiplier, min_charge, enabled, created_at, updated_at FROM model_pricing ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.ModelPricing
	for rows.Next() {
		p := models.ModelPricing{}
		if err := rows.Scan(&p.ID, &p.ModelName, &p.InputMultiplier, &p.OutputMultiplier, &p.MinCharge, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, nil
}

func (d *DB) UpdateModelPricing(p *models.ModelPricing) error {
	_, err := d.conn.Exec(`UPDATE model_pricing SET input_multiplier = ?, output_multiplier = ?, min_charge = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		p.InputMultiplier, p.OutputMultiplier, p.MinCharge, p.Enabled, p.ID)
	return err
}

func (d *DB) GetAllCoupons() ([]models.Coupon, error) {
	rows, err := d.conn.Query(`SELECT id, code, quota, days, max_uses, used_count, plan_id, expired_at, is_active, created_by, created_at FROM coupons ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Coupon
	for rows.Next() {
		c := models.Coupon{}
		var exp sql.NullTime
		if err := rows.Scan(&c.ID, &c.Code, &c.Quota, &c.Days, &c.MaxUses, &c.UsedCount, &c.PlanID, &exp, &c.IsActive, &c.CreatedBy, &c.CreatedAt); err != nil {
			return nil, err
		}
		if exp.Valid {
			c.ExpiredAt = &exp.Time
		}
		list = append(list, c)
	}
	return list, nil
}

func (d *DB) GetSystemStats() (map[string]any, error) {
	var totalKeys, activeKeys, totalQuotaIssued, totalQuotaConsumed int64
	_ = d.conn.QueryRow(`SELECT COUNT(*), SUM(CASE WHEN status = 'ACTIVE' THEN 1 ELSE 0 END), SUM(total_quota), SUM(consumed_quota) FROM virtual_keys`).
		Scan(&totalKeys, &activeKeys, &totalQuotaIssued, &totalQuotaConsumed)

	var totalProviders, activeProviders int64
	_ = d.conn.QueryRow(`SELECT COUNT(*), SUM(CASE WHEN status = 'ACTIVE' THEN 1 ELSE 0 END) FROM providers`).
		Scan(&totalProviders, &activeProviders)

	var totalTransactions int64
	_ = d.conn.QueryRow(`SELECT COUNT(*) FROM quota_transactions`).Scan(&totalTransactions)

	return map[string]any{
		"total_keys":           totalKeys,
		"active_keys":          activeKeys,
		"total_quota_issued":   totalQuotaIssued,
		"total_quota_consumed": totalQuotaConsumed,
		"total_providers":      totalProviders,
		"active_providers":     activeProviders,
		"total_transactions":   totalTransactions,
	}, nil
}
