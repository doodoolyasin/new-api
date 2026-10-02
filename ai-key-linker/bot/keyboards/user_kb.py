from aiogram.types import ReplyKeyboardMarkup, KeyboardButton, InlineKeyboardMarkup, InlineKeyboardButton
from config import PLANS, PUBLIC_BASE_URL

def get_main_keyboard(is_admin: bool = False) -> ReplyKeyboardMarkup:
    buttons = [
        [KeyboardButton(text="📊 وضعیت کلید من"), KeyboardButton(text="💎 خرید و شارژ اشتراک")],
        [KeyboardButton(text="⚙️ راهنمای اتصال سریع"), KeyboardButton(text="🎁 زیرمجموعه‌گیری و هدیه")],
        [KeyboardButton(text="🎟 ثبت کد هدیه"), KeyboardButton(text="🔄 تعویض و ابطال کلید")],
    ]
    if is_admin:
        buttons.append([KeyboardButton(text="🛠 پنل مدیریت")])
    return ReplyKeyboardMarkup(keyboard=buttons, resize_keyboard=True)

def get_plans_inline_keyboard() -> InlineKeyboardMarkup:
    buttons = []
    for plan_id, info in PLANS.items():
        if plan_id == "TRIAL":
            continue
        btn_text = f"💎 {info['name']} - {info['price_stars']} ⭐ ستاره"
        buttons.append([InlineKeyboardButton(text=btn_text, callback_data=f"buy_plan_{plan_id}")])
    buttons.append([InlineKeyboardButton(text="🔙 بازگشت به منو", callback_data="cancel_action")])
    return InlineKeyboardMarkup(inline_keyboard=buttons)

def get_setup_inline_keyboard(user_key: str) -> InlineKeyboardMarkup:
    nextchat_url = f"https://app.nextchat.dev/#/?settings={{\"key\":\"{user_key}\",\"url\":\"{PUBLIC_BASE_URL}\"}}"
    buttons = [
        [InlineKeyboardButton(text="🌐 اتصال خودکار ۱-کلیکه به NextChat", url=nextchat_url)],
        [InlineKeyboardButton(text="💻 راهنمای اتصال به Cursor / VSCode", callback_data="setup_cursor")],
        [InlineKeyboardButton(text="📱 راهنمای نرم‌افزار Chatbox", callback_data="setup_chatbox")],
    ]
    return InlineKeyboardMarkup(inline_keyboard=buttons)

def get_key_action_keyboard(key_id: str) -> InlineKeyboardMarkup:
    buttons = [
        [InlineKeyboardButton(text="🔄 ابطال و دریافت کلید جدید", callback_data=f"regen_key_{key_id}")],
        [InlineKeyboardButton(text="💎 شارژ یا ارتقای این کلید", callback_data="open_plans")],
        [InlineKeyboardButton(text="⚙️ تنظیمات اتصال ۱-کلیکه", callback_data="open_setup")],
    ]
    return InlineKeyboardMarkup(inline_keyboard=buttons)
