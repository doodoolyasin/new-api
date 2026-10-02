import asyncio
import sqlite3
import datetime
from aiogram import Bot
from services.bot_db import has_notification_sent, record_notification_sent
from config import PLANS

async def notification_worker(bot: Bot):
    while True:
        try:
            await check_and_notify_users(bot)
        except Exception as e:
            pass
        await asyncio.sleep(600) # Every 10 minutes

async def check_and_notify_users(bot: Bot):
    conn = sqlite3.connect("/root/ai-key-linker/gateway.db")
    c = conn.cursor()
    c.execute("""
    SELECT key_id, owner_id, total_quota, remain_quota, consumed_quota, expired_at 
    FROM virtual_keys 
    WHERE status = 'ACTIVE' AND owner_id LIKE 'tg_%' OR owner_id GLOB '[0-9]*'
    """)
    rows = c.fetchall()
    conn.close()

    now = datetime.datetime.now(datetime.timezone.utc)

    for row in rows:
        key_id, owner_id, total, remain, consumed, exp_str = row
        try:
            tg_id = int(owner_id.replace("tg_", ""))
        except ValueError:
            continue

        # 1. Check 80% quota usage
        if total > 0 and consumed >= (total * 0.8):
            if not has_notification_sent(key_id, "QUOTA_80_PERCENT"):
                text = (
                    "⚠️ **هشدار مصرف سهمیه توکن:**\n\n"
                    f"بیش از **۸۰٪** از سهمیه کلید هوش مصنوعی شما مصرف شده است!\n"
                    f"باقیمانده: **{remain:,}** از **{total:,}** توکن.\n\n"
                    "برای جلوگیری از قطع شدن دسترسی، می‌توانید با دستور /plans کلید خود را شارژ کنید."
                )
                try:
                    await bot.send_message(tg_id, text, parse_mode="Markdown")
                    record_notification_sent(key_id, tg_id, "QUOTA_80_PERCENT")
                except Exception:
                    pass

        # 2. Check 24-hour expiration
        if exp_str:
            try:
                exp_clean = exp_str.replace("Z", "+00:00")
                exp_time = datetime.datetime.fromisoformat(exp_clean)
                diff = exp_time - now
                if datetime.timedelta(seconds=0) < diff <= datetime.timedelta(hours=24):
                    if not has_notification_sent(key_id, "EXPIRING_24H"):
                        hours_left = int(diff.total_seconds() // 3600)
                        text = (
                            "⏳ **یادآوری اتمام زمان اعتبار کلید:**\n\n"
                            f"کمتر از **{hours_left} ساعت** تا انقضای کلید شما باقی مانده است.\n\n"
                            "جهت تمدید، از بخش «💎 خرید و شارژ اشتراک» اقدام فرمایید."
                        )
                        try:
                            await bot.send_message(tg_id, text, parse_mode="Markdown")
                            record_notification_sent(key_id, tg_id, "EXPIRING_24H")
                        except Exception:
                            pass
            except Exception:
                pass
