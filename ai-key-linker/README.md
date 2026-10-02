# AI Key Linker Platform & Smart AI Gateway

A high-performance, lightweight, production-grade AI API key distribution and management system inspired by File Linker Robot.

## Architecture

1. **Go AI Gateway / Smart AI Proxy (`gateway/`)**:
   - OpenAI compatible endpoints (`/v1/chat/completions`, `/v1/models`, `/healthz`).
   - Virtual API keys with hashed secrets (SHA-256).
   - In-memory token bucket per-key RPM rate limiting (nanosecond-latency checks, 429 response with `Retry-After`).
   - Atomic max concurrency limiter per key.
   - Max allowed IPs per key with IPv4/IPv6 normalization and trusted proxy validation.
   - Model allowlists and dynamic model pricing multipliers (e.g. 5x for Opus 5.5, 1x for DeepSeek).
   - Provider pool with priority-based smart failover across multiple upstream API keys.
   - Streaming SSE support with real-time token tracking and zero-latency passthrough.
   - Atomic quota deduction with SQLite WAL mode and transaction ledger (`quota_transactions`).
   - Zero frontend overhead — runs under 15MB of RAM!

2. **Telegram Distribution Bot (`bot/`)**:
   - Built with Python & Aiogram 3.
   - Deep-linking (`/start trial_opus`, `/start ref_USERID`, campaigns).
   - Instant trial key issuance on start.
   - User dashboard (`/status` with visual text progress bar, live quota, RPM, remaining time).
   - Atomic key regeneration (`/regenerate`) preserving remaining quota and duration.
   - Telegram Stars payments for Economic and VIP subscription plans.
   - Viral referral system with qualified invitation bonuses.
   - Coupon / gift code redemption (`/redeem`).
   - 1-click configuration for NextChat, Cursor, VSCode, LibreChat, Chatbox.
   - Background notification worker (80% quota usage alert, 24h expiration reminders).
   - Admin panel for managing keys, providers, coupons, and system stats.

## Running Services

Both services run as standard systemd units:
- `systemctl status ai-gateway.service`
- `systemctl status ai-linker-bot.service`

## Running Tests

```bash
python3 /root/ai-key-linker/test_gateway_suite.py
```
