import os

BOT_TOKEN = os.getenv("BOT_TOKEN", "8690404926:AAHfOnd7MSur0R072whf-tWiU2_bvr4rzQ0")
GATEWAY_URL = os.getenv("GATEWAY_URL", "http://127.0.0.1:8085")
PUBLIC_BASE_URL = os.getenv("PUBLIC_BASE_URL", "http://95.182.80.85:8085/v1")
ADMIN_KEY = os.getenv("GATEWAY_ADMIN_KEY", "adm_secret_key_linker_2026_x")

# Admin IDs (Telegram user IDs)
ADMIN_IDS = [int(i.strip()) for i in os.getenv("ADMIN_IDS", "1099934856").split(",") if i.strip()]

# Sponsor channels for force join verification
# Format: [{"id": "@channel_username", "name": "کانال اخبار هوش مصنوعی", "link": "https://t.me/channel_username"}]
SPONSOR_CHANNELS = []

# Bot database path for bot-specific states (referrals, notifications, payments)
BOT_DB_PATH = os.getenv("BOT_DB_PATH", "/root/ai-key-linker/bot_data.db")

# Subscription Plans definition
PLANS = {
    "TRIAL": {
        "name": "تست رایگان (Trial)",
        "quota": 10000,
        "days": 1,
        "rpm": 2,
        "concurrency": 1,
        "max_ips": 1,
        "models": "claude-opus-5.5,deepseek-chat",
        "price_stars": 0,
        "description": "۱۰ هزار توکن | ۱ روز | سرعت ۲ درخواست/دقیقه"
    },
    "ECONOMIC": {
        "name": "پلن اقتصادی (Economic)",
        "quota": 100000,
        "days": 15,
        "rpm": 8,
        "concurrency": 2,
        "max_ips": 2,
        "models": "*",
        "price_stars": 50,
        "description": "۱۰۰ هزار توکن | ۱۵ روز | سرعت ۸ درخواست/دقیقه"
    },
    "VIP": {
        "name": "پلن حرفه‌ای VIP (Developer)",
        "quota": 500000,
        "days": 30,
        "rpm": 30,
        "concurrency": 4,
        "max_ips": 3,
        "models": "*",
        "price_stars": 200,
        "description": "۵۰۰ هزار توکن | ۳۰ روز | سرعت ۳۰ درخواست/دقیقه | دسترسی کامل"
    }
}
