from aiogram import Router, F
from aiogram.filters import CommandStart, CommandObject
from aiogram.types import Message, CallbackQuery
from services.gateway_client import gateway_client
from services.bot_db import get_or_create_user, set_active_key, record_referral, qualify_referral, get_active_key
from keyboards.user_kb import get_main_keyboard, get_key_action_keyboard
from config import PLANS, ADMIN_IDS, PUBLIC_BASE_URL

router = Router()

@router.message(CommandStart())
async def cmd_start(message: Message, command: CommandObject):
    user_id = message.from_user.id
    username = message.from_user.username or ""
    first_name = message.from_user.first_name or "کاربر"
    is_admin = user_id in ADMIN_IDS

    args = command.args
    inviter_id = None

    if args and args.startswith("ref_"):
        try:
            inviter_id = int(args.replace("ref_", ""))
            if inviter_id != user_id:
                record_referral(inviter_id, user_id)
        except ValueError:
            pass

    user = get_or_create_user(user_id, username, first_name, inviter_id)
    active_key_id = user.get("active_key_id")

    # If user doesn't have an active key, issue a Trial Key automatically!
    if not active_key_id:
        trial_info = PLANS["TRIAL"]
        created = await gateway_client.create_key(
            owner_id=str(user_id),
            plan_id="TRIAL",
            quota=trial_info["quota"],
            days=trial_info["days"],
            rpm_limit=trial_info["rpm"],
            max_concurrency=trial_info["concurrency"],
            max_allowed_ips=trial_info["max_ips"],
            allowed_models=trial_info["models"],
            note=f"Auto trial for @{username}"
        )
        if created:
            active_key_id = created["key_id"]
            set_active_key(user_id, active_key_id)
            raw_secret = created.get("key_secret", "")

            # Check if this qualifies an inviter for referral reward
            bonus_inviter = qualify_referral(user_id)
            if bonus_inviter:
                inviter_key = get_active_key(bonus_inviter)
                if inviter_key:
                    await gateway_client.add_quota(inviter_key, 20000, "REFERRAL_REWARD", f"Bonus for inviting {user_id}")

            welcome_text = (
                f"سلام {first_name} عزیز! به پلتفرم توزیع هوش مصنوعی خوش آمدید. 🚀\n\n"
                f"🎁 **یک کلید تست رایگان با ۱۰,۰۰۰ توکن برای شما صادر شد:**\n\n"
                f"🔑 **کلید اختصاصی شما (Virtual API Key):**\n"
                f"`{raw_secret}`\n\n"
                f"🌐 **Base URL استاندارد:**\n"
                f"`{PUBLIC_BASE_URL}`\n\n"
                f"⚡️ **مشخصات کلید:**\n"
                f"• اعتبار: ۲۴ ساعت\n"
                f"• سرعت مجاز: ۲ درخواست در دقیقه (RPM=2)\n"
                f"• اتصالات مجاز: ۱ آی‌پی\n"
                f"• مدل‌های فعال: Claude Opus 5.5, DeepSeek\n\n"
                f"از منوی زیر می‌توانید وضعیت، راهنمای اتصال به نرم‌افزارها یا خرید پلن‌های حرفه‌ای را انتخاب کنید:"
            )
            await message.answer(welcome_text, reply_markup=get_main_keyboard(is_admin), parse_mode="Markdown")
            return

    # User already has a key
    text = (
        f"سلام مجدد {first_name} عزیز! 👋\n"
        f"پلتفرم هوشمند توزیع API فعال است.\n\n"
        f"برای مشاهده موجودی توکن، زمان انقضا و سرعت کلید خود دکمه **«📊 وضعیت کلید من»** را بزنید."
    )
    await message.answer(text, reply_markup=get_main_keyboard(is_admin), parse_mode="Markdown")
