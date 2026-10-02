package models

import (
	"time"
)

// Key Status
const (
	StatusActive    = "ACTIVE"
	StatusDisabled  = "DISABLED"
	StatusExpired   = "EXPIRED"
	StatusRevoked   = "REVOKED"
	StatusSuspended = "SUSPENDED"
	StatusExhausted = "EXHAUSTED"
)

// Ledger Transaction Types
const (
	TxInitialAllocation = "INITIAL_ALLOCATION"
	TxUsage             = "USAGE"
	TxRefund            = "REFUND"
	TxAdminAdjustment   = "ADMIN_ADJUSTMENT"
	TxPurchase          = "PURCHASE"
	TxReferralReward    = "REFERRAL_REWARD"
	TxCouponReward      = "COUPON_REWARD"
	TxExpiration        = "EXPIRATION"
)

// VirtualKey represents a client's API key
type VirtualKey struct {
	ID                  int64      `json:"id"`
	KeyID               string     `json:"key_id"`               // Public ID e.g. "vk_abc123"
	KeySecret           string     `json:"key_secret,omitempty"` // Raw secret, returned only once upon creation
	KeyHash             string     `json:"-"`                    // SHA-256 hash
	OwnerID             string     `json:"owner_id"`             // Telegram user ID or account ID
	Status              string     `json:"status"`               // ACTIVE, DISABLED, EXPIRED, REVOKED, SUSPENDED, EXHAUSTED
	PlanID              string     `json:"plan_id"`              // FREE, ECONOMIC, VIP
	Note                string     `json:"note"`
	TotalQuota          int64      `json:"total_quota"`          // In tokens or micro-units
	RemainQuota         int64      `json:"remain_quota"`
	ConsumedQuota       int64      `json:"consumed_quota"`
	RPMLimit            int        `json:"rpm_limit"`            // Requests per minute
	MaxConcurrency      int        `json:"max_concurrency"`      // Simultaneous in-flight requests
	MaxAllowedIPs       int        `json:"max_allowed_ips"`      // Distinct IPs allowed
	AllowedModels       string     `json:"allowed_models"`       // Comma-separated or "*"
	BlockedModels       string     `json:"blocked_models"`
	CurrentIPCount      int        `json:"current_ip_count"`
	LastIP              string     `json:"last_ip"`
	LastUsedAt          *time.Time `json:"last_used_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	ExpiredAt           *time.Time `json:"expired_at"`
	RevokedAt           *time.Time `json:"revoked_at"`
	RegenerationHistory string     `json:"regeneration_history"` // JSON array of past key IDs
}

// Provider represents an upstream AI provider account/key
type Provider struct {
	ID                  int64      `json:"id"`
	Name                string     `json:"name"`                 // e.g. "codecraft-1"
	BaseURL             string     `json:"base_url"`             // e.g. "https://codecraftapi.com/v1"
	APIKey              string     `json:"-"`                    // Upstream secret key
	SupportedModels     string     `json:"supported_models"`     // Comma-separated or "*"
	Priority            int        `json:"priority"`             // Lower = higher priority
	Weight              int        `json:"weight"`               // Weighted random selection
	Status              string     `json:"status"`               // ACTIVE, COOLDOWN, DISABLED
	ConsecutiveFailures int        `json:"consecutive_failures"`
	CooldownUntil       *time.Time `json:"cooldown_until"`
	LastSuccessAt       *time.Time `json:"last_success_at"`
	LastErrorAt         *time.Time `json:"last_error_at"`
	LastError           string     `json:"last_error"`
	AvgLatencyMs        int64      `json:"avg_latency_ms"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// ModelPricing represents multipliers per model
type ModelPricing struct {
	ID               int64     `json:"id"`
	ModelName        string    `json:"model_name"` // Pattern or exact name e.g. "claude-opus-5.5"
	InputMultiplier  float64   `json:"input_multiplier"`
	OutputMultiplier float64   `json:"output_multiplier"`
	MinCharge        int64     `json:"min_charge"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// QuotaTransaction represents an immutable ledger entry
type QuotaTransaction struct {
	ID        int64     `json:"id"`
	KeyID     string    `json:"key_id"`
	Type      string    `json:"type"` // USAGE, PURCHASE, REFUND, etc.
	Amount    int64     `json:"amount"` // Positive for credit, negative for debit
	Balance   int64     `json:"balance"` // Balance after transaction
	Model     string    `json:"model,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	Details   string    `json:"details,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// IPRecord tracks client IPs per key
type IPRecord struct {
	ID        int64     `json:"id"`
	KeyID     string    `json:"key_id"`
	IP        string    `json:"ip"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	Hits      int64     `json:"hits"`
}

// AuditLog tracks sensitive system operations
type AuditLog struct {
	ID         int64     `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Actor      string    `json:"actor"` // "system", "bot", "admin:123", "user:456"
	Target     string    `json:"target"`
	Action     string    `json:"action"` // "KEY_CREATED", "KEY_REVOKED", "PRICING_UPDATED", etc.
	Metadata   string    `json:"metadata"`
	RemoteAddr string    `json:"remote_addr"`
}

// Coupon / Redeem Code
type Coupon struct {
	ID         int64      `json:"id"`
	Code       string     `json:"code"`
	Quota      int64      `json:"quota"`
	Days       int        `json:"days"`
	MaxUses    int        `json:"max_uses"`
	UsedCount  int        `json:"used_count"`
	PlanID     string     `json:"plan_id"`
	ExpiredAt  *time.Time `json:"expired_at"`
	IsActive   bool       `json:"is_active"`
	CreatedBy  string     `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
}
