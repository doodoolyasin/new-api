from aiogram import Router, F
from aiogram.filters import Command
from aiogram.types import Message
from services.bot_db import get_referral_stats

router = Router()

@router.message(F.text == "🎁 زیرمجموعه‌گیری و هدیه")
@router.message(Command("referral"))
async def show_referral(message: Message):
    user_id = message.from_user.id
    bot_info = await message.bot.get_me()
    bot_username = bot_info.username

    ref_link = f"https://t.me/{bot_username}?start=ref_{user_id}"
    stats = get_referral_stats(user_id)

    text = (
        "🎁 **سیستم دعوت دوستان و دریافت توکن رایگان:**\n\n"
        "با دعوت دوستانتان به ربات، با فعال‌سازی هر کلید توسط آنها، **۲۰,۰۰۰ توکن هدیه** به صورت مستقیم به کلید شما اضافه می‌شود!\n\n"
        f"🔗 **لینک دعوت اختصاصی شما:**\n"
        f"`{ref_link}`\n\n"
        f"📊 **آمار زیرمجموعه‌های شما:**\n"
        f"• تعداد کل افراد دعوت‌شده: **{stats['total']}** نفر\n"
        f"• افراد فعال‌شده: **{stats['qualified']}** نفر\n"
        f"• توکن‌های پاداش دریافت‌شده: **{stats['qualified'] * 20000:,}** توکن\n\n"
        "لینک بالا را کپی کرده و برای دوستان یا در گروه‌ها ارسال کنید."
    )
    await message.answer(text, parse_mode="Markdown")
