package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-gateway/config"
	"ai-gateway/db"
	"ai-gateway/limiter"
	"ai-gateway/models"
	"ai-gateway/proxy"
)

type APIHandler struct {
	cfg         *config.Config
	db          *db.DB
	rpmLimiter  *limiter.RPMLimiter
	concLimiter *limiter.ConcurrencyLimiter
	relay       *proxy.RelayEngine
}

func NewAPIHandler(cfg *config.Config, database *db.DB, rpmLimiter *limiter.RPMLimiter, concLimiter *limiter.ConcurrencyLimiter, relay *proxy.RelayEngine) *APIHandler {
	return &APIHandler{
		cfg:         cfg,
		db:          database,
		rpmLimiter:  rpmLimiter,
		concLimiter: concLimiter,
		relay:       relay,
	}
}

func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.handleHealthz)
	mux.HandleFunc("/v1/models", h.handleModels)
	mux.HandleFunc("/v1/chat/completions", h.handleChatCompletions)

	// Admin and Bot Management APIs
	mux.HandleFunc("/api/v1/keys", h.handleKeys)
	mux.HandleFunc("/api/v1/keys/status", h.handleKeyStatus)
	mux.HandleFunc("/api/v1/keys/regenerate", h.handleKeyRegenerate)
	mux.HandleFunc("/api/v1/keys/revoke", h.handleKeyRevoke)
	mux.HandleFunc("/api/v1/keys/quota", h.handleKeyQuota)
	mux.HandleFunc("/api/v1/coupons/redeem", h.handleCouponRedeem)
	mux.HandleFunc("/api/v1/admin/stats", h.handleAdminStats)
	mux.HandleFunc("/api/v1/providers", h.handleProviders)
}

// Global CORS Middleware
func (h *APIHandler) WrapCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		for _, o := range h.cfg.AllowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, User-Agent")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *APIHandler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	})
}

func (h *APIHandler) handleModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	modelsList := []map[string]any{
		{"id": "claude-opus-5.5", "object": "model", "created": 1720000000, "owned_by": "anthropic"},
		{"id": "claude-sonnet-3.5", "object": "model", "created": 1720000000, "owned_by": "anthropic"},
		{"id": "deepseek-chat", "object": "model", "created": 1720000000, "owned_by": "deepseek"},
		{"id": "deepseek-coder", "object": "model", "created": 1720000000, "owned_by": "deepseek"},
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"object": "list",
		"data":   modelsList,
	})
}

func (h *APIHandler) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = generateRandomString("req_", 16)
	}
	w.Header().Set("X-Request-ID", reqID)

	// 1. Extract and validate Bearer Token
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		h.writeError(w, http.StatusUnauthorized, "invalid_api_key", "Missing or malformed Authorization header. Expected 'Bearer sk-...'")
		return
	}
	rawKey := strings.TrimPrefix(authHeader, "Bearer ")
	rawKey = strings.TrimSpace(rawKey)
	keyHash := db.HashKey(rawKey)

	// 2. Fetch Virtual Key from DB
	vk, err := h.db.GetVirtualKeyByHash(keyHash)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "invalid_api_key", "Incorrect or invalid API key provided.")
		return
	}

	// 3. Status checks
	if vk.Status == models.StatusRevoked {
		h.writeError(w, http.StatusUnauthorized, "key_revoked", "This API key has been revoked.")
		return
	}
	if vk.Status == models.StatusDisabled || vk.Status == models.StatusSuspended {
		h.writeError(w, http.StatusForbidden, "key_disabled", "This API key is suspended or disabled.")
		return
	}
	if vk.ExpiredAt != nil && time.Now().After(*vk.ExpiredAt) {
		h.writeError(w, http.StatusForbidden, "key_expired", "This API key has expired. Please renew your plan.")
		return
	}
	if vk.RemainQuota <= 0 {
		h.writeError(w, http.StatusForbidden, "insufficient_quota", "Your quota has been exhausted. Please recharge your key.")
		return
	}

	// 4. IP Check and Bounded IP History
	clientIP := limiter.ExtractClientIP(r, h.cfg.TrustedProxies)
	allowedIP, err := h.db.CheckAndRegisterIP(vk.KeyID, clientIP, vk.MaxAllowedIPs)
	if !allowedIP || err != nil {
		h.writeError(w, http.StatusForbidden, "ip_restricted", fmt.Sprintf("Access denied: key has reached maximum allowed IP limit (%d IPs).", vk.MaxAllowedIPs))
		return
	}

	// 5. RPM Rate Limiting
	allowedRPM, retryAfter := h.rpmLimiter.Allow(vk.KeyID, vk.RPMLimit)
	if !allowedRPM {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
		h.writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", fmt.Sprintf("RPM rate limit exceeded (limit: %d req/min). Try again in %d seconds.", vk.RPMLimit, int(retryAfter.Seconds())))
		return
	}

	// 6. Concurrency Limiting
	if !h.concLimiter.Acquire(vk.KeyID, vk.MaxConcurrency) {
		h.writeError(w, http.StatusTooManyRequests, "concurrency_limit_exceeded", fmt.Sprintf("Max concurrency limit reached (%d simultaneous requests in progress).", vk.MaxConcurrency))
		return
	}
	defer h.concLimiter.Release(vk.KeyID)

	// 7. Read and Parse Body
	bodyBytes, err := io.ReadAll(http.MaxBytesReader(w, r.Body, h.cfg.MaxRequestBodySize))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "request_too_large", "Request body too large or unreadable.")
		return
	}

	var chatReq proxy.ChatCompletionRequest
	if err := json.Unmarshal(bodyBytes, &chatReq); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_payload", "Invalid JSON request payload.")
		return
	}
	if chatReq.Model == "" {
		chatReq.Model = "claude-opus-5.5" // Default
	}

	// 8. Model Allowlist check
	if !isModelAllowed(chatReq.Model, vk.AllowedModels, vk.BlockedModels) {
		h.writeError(w, http.StatusForbidden, "model_not_allowed", fmt.Sprintf("Your API key does not have permission to access model '%s'.", chatReq.Model))
		return
	}

	// 9. Forward Request via Relay Engine (Smart Failover + Quota Deduction)
	if err := h.relay.ForwardRequest(r.Context(), w, r, vk, bodyBytes, &chatReq, reqID); err != nil {
		h.writeError(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
}

// Admin / Bot Management Handlers
func (h *APIHandler) verifyAdmin(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	key := strings.TrimPrefix(auth, "Bearer ")
	return key == h.cfg.AdminAPIKey
}

func (h *APIHandler) handleKeys(w http.ResponseWriter, r *http.Request) {
	if !h.verifyAdmin(r) {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid admin key")
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			OwnerID        string `json:"owner_id"`
			PlanID         string `json:"plan_id"`
			Quota          int64  `json:"quota"`
			Days           int    `json:"days"`
			RPMLimit       int    `json:"rpm_limit"`
			MaxConcurrency int    `json:"max_concurrency"`
			MaxAllowedIPs  int    `json:"max_allowed_ips"`
			AllowedModels  string `json:"allowed_models"`
			Note           string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		if req.RPMLimit <= 0 {
			req.RPMLimit = 10
		}
		if req.MaxConcurrency <= 0 {
			req.MaxConcurrency = 1
		}
		if req.MaxAllowedIPs <= 0 {
			req.MaxAllowedIPs = 2
		}
		if req.AllowedModels == "" {
			req.AllowedModels = "*"
		}

		rawSecret := "sk-link-" + generateRandomString("", 32)
		keyID := "vk_" + generateRandomString("", 16)
		keyHash := db.HashKey(rawSecret)

		var expiredAt *time.Time
		if req.Days > 0 {
			exp := time.Now().Add(time.Duration(req.Days) * 24 * time.Hour)
			expiredAt = &exp
		}

		vk := &models.VirtualKey{
			KeyID:          keyID,
			KeySecret:      rawSecret,
			KeyHash:        keyHash,
			OwnerID:        req.OwnerID,
			Status:         models.StatusActive,
			PlanID:         req.PlanID,
			Note:           req.Note,
			TotalQuota:     req.Quota,
			RemainQuota:    req.Quota,
			RPMLimit:       req.RPMLimit,
			MaxConcurrency: req.MaxConcurrency,
			MaxAllowedIPs:  req.MaxAllowedIPs,
			AllowedModels:  req.AllowedModels,
			ExpiredAt:      expiredAt,
		}

		if err := h.db.CreateVirtualKey(vk); err != nil {
			h.writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(vk)
		return
	}

	if r.Method == http.MethodGet {
		ownerID := r.URL.Query().Get("owner_id")
		keys, err := h.db.GetVirtualKeysByOwner(ownerID)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(keys)
		return
	}
}

func (h *APIHandler) handleKeyStatus(w http.ResponseWriter, r *http.Request) {
	keyID := r.URL.Query().Get("key_id")
	rawKey := r.URL.Query().Get("key")

	var vk *models.VirtualKey
	var err error

	if keyID != "" {
		vk, err = h.db.GetVirtualKeyByID(keyID)
	} else if rawKey != "" {
		vk, err = h.db.GetVirtualKeyByHash(db.HashKey(rawKey))
	} else {
		h.writeError(w, http.StatusBadRequest, "missing_parameter", "key_id or key required")
		return
	}

	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Virtual key not found")
		return
	}

	activeReqs := h.concLimiter.Current(vk.KeyID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"key_id":          vk.KeyID,
		"owner_id":        vk.OwnerID,
		"status":          vk.Status,
		"plan_id":         vk.PlanID,
		"total_quota":     vk.TotalQuota,
		"remain_quota":    vk.RemainQuota,
		"consumed_quota":  vk.ConsumedQuota,
		"rpm_limit":       vk.RPMLimit,
		"max_concurrency": vk.MaxConcurrency,
		"active_requests": activeReqs,
		"max_allowed_ips": vk.MaxAllowedIPs,
		"current_ips":     vk.CurrentIPCount,
		"allowed_models":  vk.AllowedModels,
		"expired_at":      vk.ExpiredAt,
		"created_at":      vk.CreatedAt,
	})
}

func (h *APIHandler) handleKeyRegenerate(w http.ResponseWriter, r *http.Request) {
	if !h.verifyAdmin(r) {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid admin key")
		return
	}

	var req struct {
		OldKeyID string `json:"old_key_id"`
		Actor    string `json:"actor"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	rawSecret := "sk-link-" + generateRandomString("", 32)
	newKeyID := "vk_" + generateRandomString("", 16)
	newKeyHash := db.HashKey(rawSecret)

	newKey, err := h.db.RegenerateVirtualKey(req.OldKeyID, newKeyID, newKeyHash, req.Actor)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "regenerate_failed", err.Error())
		return
	}

	newKey.KeySecret = rawSecret
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(newKey)
}

func (h *APIHandler) handleKeyRevoke(w http.ResponseWriter, r *http.Request) {
	if !h.verifyAdmin(r) {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid admin key")
		return
	}

	var req struct {
		KeyID string `json:"key_id"`
		Actor string `json:"actor"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if err := h.db.RevokeVirtualKey(req.KeyID, req.Actor); err != nil {
		h.writeError(w, http.StatusBadRequest, "revoke_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "revoked_key_id": req.KeyID})
}

func (h *APIHandler) handleKeyQuota(w http.ResponseWriter, r *http.Request) {
	if !h.verifyAdmin(r) {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid admin key")
		return
	}

	var req struct {
		KeyID   string `json:"key_id"`
		Amount  int64  `json:"amount"`
		Type    string `json:"type"`
		Details string `json:"details"`
		Actor   string `json:"actor"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	newBal, err := h.db.AddQuota(req.KeyID, req.Amount, req.Type, req.Details, req.Actor)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "add_quota_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "key_id": req.KeyID, "new_balance": newBal})
}

func (h *APIHandler) handleCouponRedeem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code   string `json:"code"`
		UserID string `json:"user_id"`
		KeyID  string `json:"key_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	quotaAwarded, err := h.db.RedeemCoupon(req.Code, req.UserID, req.KeyID, "user:"+req.UserID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "redeem_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":            true,
		"quota_awarded": quotaAwarded,
		"key_id":        req.KeyID,
	})
}

func (h *APIHandler) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	if !h.verifyAdmin(r) {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid admin key")
		return
	}

	providers, _ := h.db.GetAllProviders()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"providers": providers,
		"server_time": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *APIHandler) handleProviders(w http.ResponseWriter, r *http.Request) {
	if !h.verifyAdmin(r) {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid admin key")
		return
	}

	if r.Method == http.MethodGet {
		providers, err := h.db.GetAllProviders()
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(providers)
		return
	}

	if r.Method == http.MethodPost {
		var p models.Provider
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if p.Status == "" {
			p.Status = models.StatusActive
		}
		if err := h.db.InsertProvider(&p); err != nil {
			h.writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(p)
		return
	}
}

func (h *APIHandler) writeError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errType,
			"code":    status,
		},
	})
}

func isModelAllowed(model, allowed, blocked string) bool {
	if blocked != "" {
		for _, b := range strings.Split(blocked, ",") {
			if strings.TrimSpace(b) == model {
				return false
			}
		}
	}
	if allowed == "*" || allowed == "" {
		return true
	}
	for _, a := range strings.Split(allowed, ",") {
		if strings.TrimSpace(a) == model {
			return true
		}
	}
	return false
}

func generateRandomString(prefix string, length int) string {
	bytes := make([]byte, length/2)
	_, _ = rand.Read(bytes)
	return prefix + hex.EncodeToString(bytes)
}
