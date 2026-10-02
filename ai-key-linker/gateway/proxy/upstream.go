package proxy

import (
	"errors"
	"math/rand"
	"sync"
	"time"

	"ai-gateway/db"
	"ai-gateway/models"
)

var (
	ErrNoEligibleProvider = errors.New("no healthy upstream provider available for this model")
)

type UpstreamPool struct {
	db *db.DB
	mu sync.RWMutex
}

func NewUpstreamPool(database *db.DB) *UpstreamPool {
	return &UpstreamPool{
		db: database,
	}
}

// SelectProvider picks the best provider for the model using priority and weighted random
func (p *UpstreamPool) SelectProvider(model string, excludeIDs map[int64]bool) (*models.Provider, error) {
	providers, err := p.db.GetActiveProviders(model)
	if err != nil {
		return nil, err
	}

	var candidates []models.Provider
	for _, prov := range providers {
		if excludeIDs[prov.ID] {
			continue
		}
		// If in cooldown, check if time expired
		if prov.CooldownUntil != nil && time.Now().Before(*prov.CooldownUntil) {
			continue
		}
		candidates = append(candidates, prov)
	}

	if len(candidates) == 0 {
		return nil, ErrNoEligibleProvider
	}

	// Group by highest priority (lowest number)
	bestPriority := candidates[0].Priority
	var topTier []models.Provider
	for _, c := range candidates {
		if c.Priority < bestPriority {
			bestPriority = c.Priority
			topTier = []models.Provider{c}
		} else if c.Priority == bestPriority {
			topTier = append(topTier, c)
		}
	}

	// Weighted selection among top tier
	if len(topTier) == 1 {
		return &topTier[0], nil
	}

	totalWeight := 0
	for _, c := range topTier {
		w := c.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}

	r := rand.Intn(totalWeight)
	accum := 0
	for _, c := range topTier {
		w := c.Weight
		if w <= 0 {
			w = 1
		}
		accum += w
		if r < accum {
			return &c, nil
		}
	}

	return &topTier[0], nil
}

func (p *UpstreamPool) RecordSuccess(providerID int64, latencyMs int64) {
	p.db.UpdateProviderSuccess(providerID, latencyMs)
}

func (p *UpstreamPool) RecordFailure(providerID int64, errMsg string, statusCode int) {
	// Base cooldown: 30s, doubles on consecutive failures up to 300s
	cooldown := 30
	if statusCode == 429 {
		cooldown = 60 // Rate limited: wait at least 60s
	}
	p.db.UpdateProviderFailure(providerID, errMsg, cooldown)
}
