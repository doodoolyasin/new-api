import datetime
from aiogram import Router, F
from aiogram.filters import Command
from aiogram.types import Message, CallbackQuery
from services.gateway_client import gateway_client
from services.bot_db import get_active_key, set_active_key
from keyboards.user_kb import get_key_action_keyboard, get_main_keyboard
from config import ADMIN_IDS, PUBLIC_BASE_URL

router = Router()

def make_progress_bar(remain: int, total: int, length: int = 10) -> str:
    if total <= 0:
        return "[░░░░░░░░░░] 0%"
    pct = max(0, min(100, int((remain / total) * 100)))
    filled = int((pct / 100) * length)
    bar = "█" * filled + "░" * (length - filled)
    return f"[{bar}] {pct}%"

def format_remaining_time(exp_str: str) -> str:
    if not exp_str:
        return "نامحدود / بدون انقضا"
    try:
        # ISO parse
        exp_clean = exp_str.replace("Z", "+00:00")
        exp_time = datetime.datetime.fromisoformat(exp_clean)
        now = datetime.datetime.now(datetime.timezone.utc)
        diff = exp_time - now
        if diff.total_seconds() <= 0:
            return "❌ منقضی شده"
        days = diff.days
        hours = int(diff.seconds // 3600)
        return f"{days} روز و {hours} ساعت دیگر"
    except Exception:
        return exp_str[:10]

@router.message(F.text == "📊 وضعیت کلید من")
@router.message(Command("status"))
async def show_status(message: Message):
    user_id = message.from_user.id
    key_id = get_active_key(user_id)
    if not key_id:
        await message.answer("⚠️ شما در حال حاضر کلید فعالی ندارید. لطفاً دستور /start را ارسال کنید.")
        return

    data = await gateway_client.get_key_status(key_id)
    if not data:
        await message.answer("⚠️ خطا در دریافت اطلاعات کلید از گیت‌وی.")
        return

    remain = data.get("remain_quota", 0)
    total = data.get("total_quota", 0)
    consumed = data.get("consumed_quota", 0)
    rpm = data.get("rpm_limit", 0)
    max_ips = data.get("max_allowed_ips", 1)
    current_ips = data.get("current_ips", 0)
    conc = data.get("max_concurrency", 1)
    active_reqs = data.get("active_requests", 0)
    plan = data.get("plan_id", "CUSTOM")
    status = data.get("status", "UNKNOWN")
    exp_str = data.get("expired_at")

    bar_str = make_progress_bar(remain, total)
    time_str = format_remaining_time(exp_str)

    status_emoji = "🟢 فعال" if status == "ACTIVE" else f"🔴 {status}"

    text = (
        f"📊 **داشبورد وضعیت کلید هوش مصنوعی شما:**\n\n"
        f"🔑 **شناسه کلید:** `{key_id}`\n"
        f"💎 **پلن:** {plan} ({status_emoji})\n\n"
        f"📉 **سهمیه توکن باقیمانده:**\n"
        f"• باقیمانده: **{remain:,}** از **{total:,}** توکن\n"
        f"• مصرف شده: **{consumed:,}** توکن\n"
        f"• درصد سهمیه: `{bar_str}`\n\n"
        f"⏳ **اعتبار زمانی:** {time_str}\n\n"
        f"⚡️ **محدودیت‌های سرعت و شبکه:**\n"
        f"• سرعت مجاز: **{rpm}** درخواست در دقیقه (RPM)\n"
        f"• اتصالات همزمان: **{active_reqs}** از **{conc}** همزمان\n"
        f"• تعداد آی‌پی فعال: **{current_ips}** از **{max_ips}** آی‌پی\n\n"
        f"🌐 **Base URL:** `{PUBLIC_BASE_URL}`"
    )

    await message.answer(text, reply_markup=get_key_action_keyboard(key_id), parse_mode="Markdown")

@router.message(F.text == "🔄 تعویض و ابطال کلید")
@router.message(Command("regenerate"))
async def cmd_regenerate(message: Message):
    user_id = message.from_user.id
    key_id = get_active_key(user_id)
    if not key_id:
        await message.answer("⚠️ شما هیچ کلید فعالی ندارید.")
        return

    res = await gateway_client.regenerate_key(key_id, actor=f"user:{user_id}")
    if not res:
        await message.answer("❌ خطا در صدور کلید جدید. لطفاً بعداً تلاش کنید.")
        return

    new_key_id = res["key_id"]
    new_secret = res["key_secret"]
    set_active_key(user_id, new_key_id)

    text = (
        f"✅ **کلید قبلی باطل شد و کلید جدید با موفقیت صادر گردید!**\n\n"
        f"سهمیه باقیمانده و زمان اعتبار شما دقیقاً حفظ شده است.\n\n"
        f"🔑 **کلید جدید شما (Secret Key):**\n"
        f"`{new_secret}`\n\n"
        f"شناسه جدید: `{new_key_id}`\n\n"
        f"⚠️ لطفاً این کلید را در تنظیمات نرم‌افزارهای خود (Cursor/NextChat/VSCode) جایگزین کنید."
    )
    await message.answer(text, parse_mode="Markdown")

@router.callback_query(F.data.startswith("regen_key_"))
async def cb_regenerate(callback: CallbackQuery):
    user_id = callback.from_user.id
    key_id = callback.data.replace("regen_key_", "")

    res = await gateway_client.regenerate_key(key_id, actor=f"user:{user_id}")
    if not res:
        await callback.answer("خطا در صدور کلید جدید!", show_alert=True)
        return

    new_key_id = res["key_id"]
    new_secret = res["key_secret"]
    set_active_key(user_id, new_key_id)

    text = (
        f"✅ **کلید با موفقیت تعویض شد!**\n\n"
        f"🔑 **کلید جدید شما:**\n"
        f"`{new_secret}`\n\n"
        f"سهمیه و زمان اشتراک شما بدون تغییر منتقل گردید."
    )
    await callback.message.answer(text, parse_mode="Markdown")
    await callback.answer()
