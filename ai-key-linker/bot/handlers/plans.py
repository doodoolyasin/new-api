from aiogram import Router, F
from aiogram.filters import Command
from aiogram.types import Message, CallbackQuery, PreCheckoutQuery, LabeledPrice
from services.gateway_client import gateway_client
from services.bot_db import get_active_key, set_active_key
from keyboards.user_kb import get_plans_inline_keyboard
from config import PLANS, PUBLIC_BASE_URL

router = Router()

@router.message(F.text == "💎 خرید و شارژ اشتراک")
@router.message(Command("plans"))
async def show_plans(message: Message):
    text = (
        "💎 **پلن‌های اشتراکی اختصاصی API هوش مصنوعی:**\n\n"
        "⚡️ تمام پلن‌ها دارای دسترسی مستقیم به **Claude Opus 5.5** و سایر مدل‌ها بدون واسطه هستند.\n\n"
        "۱. **پلن اقتصادی (Economic):**\n"
        "• ۱۰۰,۰۰۰ توکن\n"
        "• مدت اعتبار: ۱۵ روز\n"
        "• سرعت: ۸ درخواست در دقیقه (RPM=8)\n"
        "• قیمت: **۵۰ ستاره تلگرام (Telegram Stars)**\n\n"
        "۲. **پلن برنامه‌نویسان VIP (Developer):**\n"
        "• ۵۰۰,۰۰۰ توکن\n"
        "• مدت اعتبار: ۳۰ روز\n"
        "• سرعت: ۳۰ درخواست در دقیقه (RPM=30)\n"
        "• همزمانی: ۴ اتصال همزمان | ۳ آی‌پی مجاز\n"
        "• قیمت: **۲۰۰ ستاره تلگرام (Telegram Stars)**\n\n"
        "برای خرید یا تمدید، یکی از پلن‌های زیر را انتخاب کنید:"
    )
    await message.answer(text, reply_markup=get_plans_inline_keyboard(), parse_mode="Markdown")

@router.callback_query(F.data == "open_plans")
async def cb_open_plans(callback: CallbackQuery):
    await show_plans(callback.message)
    await callback.answer()

@router.callback_query(F.data.startswith("buy_plan_"))
async def cb_buy_plan(callback: CallbackQuery):
    plan_id = callback.data.replace("buy_plan_", "")
    plan = PLANS.get(plan_id)
    if not plan:
        await callback.answer("پلن نامعتبر است!", show_alert=True)
        return

    price_stars = plan["price_stars"]
    prices = [LabeledPrice(label=plan["name"], amount=price_stars)]

    try:
        await callback.message.answer_invoice(
            title=f"خرید {plan['name']}",
            description=f"افزایش {plan['quota']:,} توکن به کلید شما با مدت {plan['days']} روز و سرعت {plan['rpm']} RPM",
            payload=f"plan_{plan_id}_{callback.from_user.id}",
            currency="XTR", # Official Telegram Stars currency
            prices=prices,
            provider_token="" # Telegram Stars requires empty provider_token
        )
        await callback.answer()
    except Exception as e:
        await callback.message.answer(f"⚠️ درگاه پرداخت ستاره‌ها موقتاً در دسترس نیست یا تلگرام پاسخ نداد: {e}")
        await callback.answer()

@router.pre_checkout_query()
async def process_pre_checkout(pre_checkout_query: PreCheckoutQuery):
    # Idempotent server-side validation
    await pre_checkout_query.answer(ok=True)

@router.message(F.successful_payment)
async def process_successful_payment(message: Message):
    payment = message.successful_payment
    payload = payment.invoice_payload # e.g. "plan_ECONOMIC_123456"
    parts = payload.split("_")
    plan_id = parts[1] if len(parts) > 1 else "ECONOMIC"
    user_id = message.from_user.id
    username = message.from_user.username or ""

    plan = PLANS.get(plan_id, PLANS["ECONOMIC"])

    active_key = get_active_key(user_id)
    if active_key:
        # Add quota to existing key
        new_balance = await gateway_client.add_quota(
            active_key,
            amount=plan["quota"],
            tx_type="PURCHASE",
            details=f"Purchased {plan['name']} via Telegram Stars ({payment.total_amount} XTR)",
            actor=f"user:{user_id}"
        )
        await message.answer(
            f"🎉 **پرداخت شما با موفقیت انجام شد!**\n\n"
            f"💎 پلن: **{plan['name']}**\n"
            f"⚡️ مقدار **{plan['quota']:,}** توکن به کلید شما اضافه شد.\n"
            f"📈 موجودی جدید: **{new_balance:,}** توکن\n\n"
            f"برای مشاهده وضعیت دکمه «📊 وضعیت کلید من» را بزنید.",
            parse_mode="Markdown"
        )
    else:
        # Create brand new key
        created = await gateway_client.create_key(
            owner_id=str(user_id),
            plan_id=plan_id,
            quota=plan["quota"],
            days=plan["days"],
            rpm_limit=plan["rpm"],
            max_concurrency=plan["concurrency"],
            max_allowed_ips=plan["max_ips"],
            allowed_models=plan["models"],
            note=f"Paid {plan_id} by @{username}"
        )
        if created:
            set_active_key(user_id, created["key_id"])
            await message.answer(
                f"🎉 **پرداخت شما تایید و کلید VIP صادر شد!**\n\n"
                f"🔑 **کلید شما:**\n`{created.get('key_secret')}`\n\n"
                f"🌐 Base URL: `{PUBLIC_BASE_URL}`\n"
                f"💎 سهمیه: **{plan['quota']:,}** توکن",
                parse_mode="Markdown"
            )
