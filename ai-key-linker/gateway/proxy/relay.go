package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ai-gateway/config"
	"ai-gateway/db"
	"ai-gateway/models"
)

type RelayEngine struct {
	db       *db.DB
	pool     *UpstreamPool
	cfg      *config.Config
	client   *http.Client
}

type ChatCompletionRequest struct {
	Model       string          `json:"model"`
	Messages    json.RawMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Extra       map[string]any  `json:"-"`
}

type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []any  `json:"choices"`
	Usage   *Usage `json:"usage,omitempty"`
}

type Usage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

func NewRelayEngine(database *db.DB, pool *UpstreamPool, cfg *config.Config) *RelayEngine {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // Preserve raw streaming
	}
	return &RelayEngine{
		db:   database,
		pool: pool,
		cfg:  cfg,
		client: &http.Client{
			Transport: transport,
			Timeout:   cfg.RequestTimeout,
		},
	}
}

// ForwardRequest handles the entire proxying lifecycle, including failover between upstream providers
func (r *RelayEngine) ForwardRequest(
	ctx context.Context,
	w http.ResponseWriter,
	req *http.Request,
	vk *models.VirtualKey,
	reqBody []byte,
	chatReq *ChatCompletionRequest,
	reqID string,
) error {
	modelPricing, _ := r.db.GetModelPricing(chatReq.Model)
	excludedProviders := make(map[int64]bool)
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		provider, err := r.pool.SelectProvider(chatReq.Model, excludedProviders)
		if err != nil {
			return fmt.Errorf("provider selection failed: %w", err)
		}

		startTime := time.Now()
		targetURL := strings.TrimRight(provider.BaseURL, "/") + "/chat/completions"

		upstreamReq, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(reqBody))
		if err != nil {
			return err
		}

		upstreamReq.Header.Set("Content-Type", "application/json")
		upstreamReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
		upstreamReq.Header.Set("X-Request-ID", reqID)

		resp, err := r.client.Do(upstreamReq)
		if err != nil {
			// Network error or timeout: failover to next provider
			r.pool.RecordFailure(provider.ID, err.Error(), 504)
			excludedProviders[provider.ID] = true
			continue
		}

		latency := time.Since(startTime).Milliseconds()

		// Provider-side error (429, 500, 502, 503): Failover!
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			r.pool.RecordFailure(provider.ID, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)), resp.StatusCode)
			excludedProviders[provider.ID] = true
			continue
		}

		// Client error (400, 401, 403, 404): DO NOT failover. Pass through directly
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			r.pool.RecordSuccess(provider.ID, latency)
			defer resp.Body.Close()
			for k, v := range resp.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(resp.StatusCode)
			_, _ = io.Copy(w, resp.Body)
			return nil
		}

		// Success!
		r.pool.RecordSuccess(provider.ID, latency)

		if chatReq.Stream {
			return r.handleStream(w, resp, vk, chatReq.Model, modelPricing, reqID)
		} else {
			return r.handleNonStream(w, resp, vk, chatReq.Model, modelPricing, reqID)
		}
	}

	return fmt.Errorf("all eligible upstream providers failed after %d retries", maxRetries)
}

func (r *RelayEngine) handleNonStream(
	w http.ResponseWriter,
	resp *http.Response,
	vk *models.VirtualKey,
	model string,
	pricing *models.ModelPricing,
	reqID string,
) error {
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Parse usage to calculate exact quota
	var completionResp ChatCompletionResponse
	var totalTokens int64 = 100 // Safe default
	if err := json.Unmarshal(bodyBytes, &completionResp); err == nil && completionResp.Usage != nil {
		inTokens := float64(completionResp.Usage.PromptTokens) * pricing.InputMultiplier
		outTokens := float64(completionResp.Usage.CompletionTokens) * pricing.OutputMultiplier
		totalTokens = int64(inTokens + outTokens)
		if totalTokens < pricing.MinCharge {
			totalTokens = pricing.MinCharge
		}
	}

	// Deduct quota
	_, _ = r.db.DeductQuota(vk.KeyID, totalTokens, model, reqID)

	// Write response headers and body
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	_, err = w.Write(bodyBytes)
	return err
}

func (r *RelayEngine) handleStream(
	w http.ResponseWriter,
	resp *http.Response,
	vk *models.VirtualKey,
	model string,
	pricing *models.ModelPricing,
	reqID string,
) error {
	defer resp.Body.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming unsupported by client")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	scanner := bufio.NewScanner(resp.Body)
	var estimatedChars int64 = 0
	var finalUsage *Usage

	for scanner.Scan() {
		line := scanner.Text()
		_, _ = fmt.Fprintf(w, "%s\n", line)
		flusher.Flush()

		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			if strings.TrimSpace(payload) == "[DONE]" {
				continue
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
				Usage *Usage `json:"usage"`
			}

			if err := json.Unmarshal([]byte(payload), &chunk); err == nil {
				if chunk.Usage != nil {
					finalUsage = chunk.Usage
				}
				for _, c := range chunk.Choices {
					estimatedChars += int64(len(c.Delta.Content))
				}
			}
		}
	}

	// Determine final tokens
	var totalTokens int64
	if finalUsage != nil && finalUsage.TotalTokens > 0 {
		inTokens := float64(finalUsage.PromptTokens) * pricing.InputMultiplier
		outTokens := float64(finalUsage.CompletionTokens) * pricing.OutputMultiplier
		totalTokens = int64(inTokens + outTokens)
	} else {
		// Estimate: ~4 characters per token
		estTokens := (estimatedChars / 4) + 20 // baseline 20 prompt tokens
		totalTokens = int64(float64(estTokens) * pricing.OutputMultiplier)
	}

	if totalTokens < pricing.MinCharge {
		totalTokens = pricing.MinCharge
	}

	// Deduct quota after stream finishes
	_, _ = r.db.DeductQuota(vk.KeyID, totalTokens, model, reqID)

	return scanner.Err()
}
