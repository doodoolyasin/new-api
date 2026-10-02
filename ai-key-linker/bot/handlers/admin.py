import sqlite3
from aiogram import Router, F
from aiogram.filters import Command, CommandObject
from aiogram.types import Message
from services.gateway_client import gateway_client
from config import ADMIN_IDS, BOT_DB_PATH

router = Router()

def is_admin(user_id: int) -> bool:
    return user_id in ADMIN_IDS

@router.message(F.text == "🛠 پنل مدیریت")
@router.message(Command("admin"))
async def admin_panel(message: Message):
    if not is_admin(message.from_user.id):
        return

    stats = await gateway_client.get_admin_stats()
    providers = stats.get("providers", []) if stats else []

    prov_lines = []
    for p in providers:
        st = "🟢" if p["status"] == "ACTIVE" else f"🔴 ({p['status']})"
        prov_lines.append(f"• **{p['name']}**: {st} | اولویت: {p['priority']} | خطاها: {p['consecutive_failures']}")

    prov_text = "\n".join(prov_lines) if prov_lines else "هیچ پرووایدی تعریف نشده"

    # User count from bot_db
    conn = sqlite3.connect(BOT_DB_PATH)
    c = conn.cursor()
    c.execute("SELECT COUNT(*) FROM users")
    total_users = c.fetchone()[0]
    conn.close()

    text = (
        "🛠 **پنل مدیریت ارشد AI Gateway:**\n\n"
        f"👥 **تعداد کاربران ربات:** {total_users:,} کاربر\n\n"
        "📡 **وضعیت ارائه‌دهندگان بالادست (Upstream Pool):**\n"
        f"{prov_text}\n\n"
        "📋 **دستورات مدیریتی:**\n"
        "• افزایش موجودی کلید: `/addquota KEY_ID AMOUNT`\n"
        "• ابطال یک کلید: `/revokekey KEY_ID`\n"
        "• ساخت کد هدیه: `/makecoupon CODE QUOTA DAYS USES`"
    )
    await message.answer(text, parse_mode="Markdown")

@router.message(Command("addquota"))
async def cmd_addquota(message: Message, command: CommandObject):
    if not is_admin(message.from_user.id):
        return
    args = command.args
    if not args or len(args.split()) < 2:
        await message.answer("فرمت: `/addquota KEY_ID AMOUNT`\nمثال: `/addquota vk_123 50000`", parse_mode="Markdown")
        return

    parts = args.split()
    key_id = parts[0]
    try:
        amount = int(parts[1])
    except ValueError:
        await message.answer("مقدار توکن باید عدد باشد.")
        return

    res = await gateway_client.add_quota(key_id, amount, "ADMIN_ADJUSTMENT", f"Admin {message.from_user.id} credited", actor=f"admin:{message.from_user.id}")
    if res is not None:
        await message.answer(f"✅ با موفقیت **{amount:,}** توکن به کلید `{key_id}` اضافه شد. موجودی جدید: **{res:,}**", parse_mode="Markdown")
    else:
        await message.answer("❌ خطا در افزایش موجودی. کلید یافت نشد.")

@router.message(Command("revokekey"))
async def cmd_revokekey(message: Message, command: CommandObject):
    if not is_admin(message.from_user.id):
        return
    key_id = command.args
    if not key_id:
        await message.answer("فرمت: `/revokekey KEY_ID`", parse_mode="Markdown")
        return

    success = await gateway_client.revoke_key(key_id.strip(), actor=f"admin:{message.from_user.id}")
    if success:
        await message.answer(f"✅ کلید `{key_id}` با موفقیت باطل (Revoke) شد.", parse_mode="Markdown")
    else:
        await message.answer("❌ خطا در ابطال کلید.")

@router.message(Command("makecoupon"))
async def cmd_makecoupon(message: Message, command: CommandObject):
    if not is_admin(message.from_user.id):
        return
    args = command.args
    if not args or len(args.split()) < 4:
        await message.answer("فرمت: `/makecoupon CODE QUOTA DAYS USES`\nمثال: `/makecoupon NEWYEAR 50000 30 100`", parse_mode="Markdown")
        return

    parts = args.split()
    code = parts[0].upper()
    try:
        quota = int(parts[1])
        days = int(parts[2])
        uses = int(parts[3])
    except ValueError:
        await message.answer("پارامترهای quota, days, uses باید عدد باشند.")
        return

    conn = sqlite3.connect("/root/ai-key-linker/gateway.db")
    c = conn.cursor()
    try:
        c.execute("""
        INSERT OR REPLACE INTO coupons (code, quota, days, max_uses, used_count, is_active, created_by)
        VALUES (?, ?, ?, ?, 0, 1, ?)
        """, (code, quota, days, uses, f"admin:{message.from_user.id}"))
        conn.commit()
        conn.close()
        await message.answer(
            f"✅ **کد هدیه با موفقیت ساخته شد!**\n\n"
            f"🎟 کد: `{code}`\n"
            f"⚡️ سهمیه: **{quota:,}** توکن\n"
            f"⏳ مدت: **{days}** روز\n"
            f"👥 ظرفیت استفاده: **{uses}** نفر",
            parse_mode="Markdown"
        )
    except Exception as e:
        conn.close()
        await message.answer(f"❌ خطا در ساخت کوپن: {e}")
