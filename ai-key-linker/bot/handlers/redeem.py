from aiogram import Router, F
from aiogram.filters import Command, CommandObject
from aiogram.types import Message
from services.gateway_client import gateway_client
from services.bot_db import get_active_key

router = Router()

@router.message(Command("redeem"))
async def cmd_redeem(message: Message, command: CommandObject):
    user_id = message.from_user.id
    code = command.args

    if not code:
        await message.answer(
            "🎟 **ثبت کد هدیه و شارژ حساب:**\n\n"
            "لطفاً کد هدیه خود را همراه با دستور ارسال کنید:\n"
            "`/redeem YOUR_CODE`\n\n"
            "مثال: `/redeem GIFT1000`",
            parse_mode="Markdown"
        )
        return

    active_key = get_active_key(user_id)
    if not active_key:
        await message.answer("⚠️ شما هنوز هیچ کلید فعالی ندارید. ابتدا با زدن /start کلید رایگان خود را دریافت کنید.")
        return

    res = await gateway_client.redeem_coupon(code, str(user_id), active_key)
    if not res:
        await message.answer("❌ کد هدیه نامعتبر است، قبلاً استفاده شده یا منقضی گردیده است.")
        return

    quota_awarded = res.get("quota_awarded", 0)
    await message.answer(
        f"🎉 **کد هدیه با موفقیت فعال شد!**\n\n"
        f"⚡️ مقدار **{quota_awarded:,}** توکن به کلید شما اضافه گردید.\n\n"
        f"برای مشاهده موجودی جدید دکمه «📊 وضعیت کلید من» را بزنید.",
        parse_mode="Markdown"
    )

@router.message(F.text == "🎟 ثبت کد هدیه")
async def show_redeem_prompt(message: Message):
    await message.answer(
        "🎟 **ثبت کد هدیه و شارژ حساب:**\n\n"
        "کد هدیه خود را به صورت زیر ارسال کنید:\n"
        "`/redeem YOUR_CODE`\n\n"
        "مثال: `/redeem GIFT1000`",
        parse_mode="Markdown"
    )
