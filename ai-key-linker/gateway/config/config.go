package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port               string
	DBPath             string
	AdminAPIKey        string
	AllowedOrigins     []string
	TrustedProxies     []string
	RequestTimeout     time.Duration
	ConnectTimeout     time.Duration
	StreamIdleTimeout  time.Duration
	MaxRequestBodySize int64
}

func LoadConfig() *Config {
	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8085"
	}

	dbPath := os.Getenv("GATEWAY_DB_PATH")
	if dbPath == "" {
		dbPath = "/root/ai-key-linker/gateway.db"
	}

	adminKey := os.Getenv("GATEWAY_ADMIN_KEY")
	if adminKey == "" {
		adminKey = "adm_secret_key_linker_2026_x"
	}

	originsStr := os.Getenv("GATEWAY_CORS_ORIGINS")
	var origins []string
	if originsStr != "" {
		for _, o := range strings.Split(originsStr, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
	}
	if len(origins) == 0 {
		origins = []string{"*"}
	}

	trustedStr := os.Getenv("GATEWAY_TRUSTED_PROXIES")
	var trusted []string
	if trustedStr != "" {
		for _, t := range strings.Split(trustedStr, ",") {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				trusted = append(trusted, trimmed)
			}
		}
	}
	if len(trusted) == 0 {
		trusted = []string{"127.0.0.1", "::1"}
	}

	reqTimeoutSec, _ := strconv.Atoi(os.Getenv("GATEWAY_REQUEST_TIMEOUT"))
	if reqTimeoutSec <= 0 {
		reqTimeoutSec = 120
	}

	maxBodyMB, _ := strconv.Atoi(os.Getenv("GATEWAY_MAX_BODY_MB"))
	if maxBodyMB <= 0 {
		maxBodyMB = 10
	}

	return &Config{
		Port:               port,
		DBPath:             dbPath,
		AdminAPIKey:        adminKey,
		AllowedOrigins:     origins,
		TrustedProxies:     trusted,
		RequestTimeout:     time.Duration(reqTimeoutSec) * time.Second,
		ConnectTimeout:     10 * time.Second,
		StreamIdleTimeout:  60 * time.Second,
		MaxRequestBodySize: int64(maxBodyMB) * 1024 * 1024,
	}
}
