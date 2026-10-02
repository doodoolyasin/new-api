import sqlite3
import datetime
from config import BOT_DB_PATH

def init_bot_db():
    conn = sqlite3.connect(BOT_DB_PATH)
    c = conn.cursor()
    
    c.execute("""
    CREATE TABLE IF NOT EXISTS users (
        user_id INTEGER PRIMARY KEY,
        username TEXT,
        first_name TEXT,
        active_key_id TEXT,
        inviter_id INTEGER,
        is_verified BOOLEAN DEFAULT 1,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    """)

    c.execute("""
    CREATE TABLE IF NOT EXISTS referrals (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        inviter_id INTEGER NOT NULL,
        invited_id INTEGER UNIQUE NOT NULL,
        status TEXT DEFAULT 'PENDING',
        reward_issued BOOLEAN DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    """)

    c.execute("""
    CREATE TABLE IF NOT EXISTS notifications (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        key_id TEXT NOT NULL,
        user_id INTEGER NOT NULL,
        event TEXT NOT NULL,
        sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(key_id, event)
    );
    """)

    c.execute("""
    CREATE TABLE IF NOT EXISTS payments (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        payment_id TEXT UNIQUE NOT NULL,
        user_id INTEGER NOT NULL,
        plan_id TEXT NOT NULL,
        amount INTEGER NOT NULL,
        currency TEXT DEFAULT 'XTR',
        status TEXT DEFAULT 'PENDING',
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    """)

    conn.commit()
    conn.close()

def get_or_create_user(user_id: int, username: str, first_name: str, inviter_id: int = None):
    conn = sqlite3.connect(BOT_DB_PATH)
    c = conn.cursor()
    c.execute("SELECT user_id, active_key_id, inviter_id FROM users WHERE user_id = ?", (user_id,))
    row = c.fetchone()
    if not row:
        c.execute("INSERT INTO users (user_id, username, first_name, inviter_id) VALUES (?, ?, ?, ?)",
                  (user_id, username, first_name, inviter_id))
        conn.commit()
        conn.close()
        return {"user_id": user_id, "active_key_id": None, "inviter_id": inviter_id, "is_new": True}
    conn.close()
    return {"user_id": row[0], "active_key_id": row[1], "inviter_id": row[2], "is_new": False}

def set_active_key(user_id: int, key_id: str):
    conn = sqlite3.connect(BOT_DB_PATH)
    c = conn.cursor()
    c.execute("UPDATE users SET active_key_id = ? WHERE user_id = ?", (key_id, user_id))
    conn.commit()
    conn.close()

def get_active_key(user_id: int):
    conn = sqlite3.connect(BOT_DB_PATH)
    c = conn.cursor()
    c.execute("SELECT active_key_id FROM users WHERE user_id = ?", (user_id,))
    row = c.fetchone()
    conn.close()
    return row[0] if row else None

def record_referral(inviter_id: int, invited_id: int) -> bool:
    if inviter_id == invited_id:
        return False
    conn = sqlite3.connect(BOT_DB_PATH)
    c = conn.cursor()
    try:
        c.execute("INSERT INTO referrals (inviter_id, invited_id, status) VALUES (?, ?, 'PENDING')",
                  (inviter_id, invited_id))
        conn.commit()
        conn.close()
        return True
    except sqlite3.IntegrityError:
        conn.close()
        return False

def qualify_referral(invited_id: int) -> int:
    """Qualifies referral and returns inviter_id if eligible for reward"""
    conn = sqlite3.connect(BOT_DB_PATH)
    c = conn.cursor()
    c.execute("SELECT inviter_id, status, reward_issued FROM referrals WHERE invited_id = ?", (invited_id,))
    row = c.fetchone()
    if row and not row[2]:
        inviter_id = row[0]
        c.execute("UPDATE referrals SET status = 'QUALIFIED', reward_issued = 1 WHERE invited_id = ?", (invited_id,))
        conn.commit()
        conn.close()
        return inviter_id
    conn.close()
    return 0

def get_referral_stats(user_id: int):
    conn = sqlite3.connect(BOT_DB_PATH)
    c = conn.cursor()
    c.execute("SELECT COUNT(*), SUM(CASE WHEN status = 'QUALIFIED' THEN 1 ELSE 0 END) FROM referrals WHERE inviter_id = ?", (user_id,))
    row = c.fetchone()
    total = row[0] or 0
    qualified = row[1] or 0
    conn.close()
    return {"total": total, "qualified": qualified}

def has_notification_sent(key_id: str, event: str) -> bool:
    conn = sqlite3.connect(BOT_DB_PATH)
    c = conn.cursor()
    c.execute("SELECT 1 FROM notifications WHERE key_id = ? AND event = ?", (key_id, event))
    row = c.fetchone()
    conn.close()
    return bool(row)

def record_notification_sent(key_id: str, user_id: int, event: str):
    conn = sqlite3.connect(BOT_DB_PATH)
    c = conn.cursor()
    c.execute("INSERT OR IGNORE INTO notifications (key_id, user_id, event) VALUES (?, ?, ?)",
              (key_id, user_id, event))
    conn.commit()
    conn.close()
