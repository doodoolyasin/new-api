from aiogram import Router, F
from aiogram.filters import Command
from aiogram.types import Message, CallbackQuery
from services.bot_db import get_active_key
from keyboards.user_kb import get_setup_inline_keyboard
from config import PUBLIC_BASE_URL

router = Router()

@router.message(F.text == "⚙️ راهنمای اتصال سریع")
@router.message(Command("setup"))
async def show_setup_guide(message: Message):
    user_id = message.from_user.id
    active_key = get_active_key(user_id) or "کلید_شما"

    text = (
        "⚙️ **راهنمای اتصال کلید هوش مصنوعی به نرم‌افزارها:**\n\n"
        "شما می‌توانید از این کلید در تمامی ابزارها و نرم‌افزارهای استاندارد OpenAI استفاده کنید:\n\n"
        f"🌐 **Base URL:**\n`{PUBLIC_BASE_URL}`\n\n"
        "🎯 **مدل‌های پیشنهادی:**\n"
        "• `claude-opus-5.5` (هوش استدلالی فوق‌العاده قوی)\n"
        "• `claude-sonnet-3.5` (کدنویسی و سرعت بالا)\n"
        "• `deepseek-chat` (اقتصادی و عمومی)\n\n"
        "یکی از گزینه‌های زیر را برای آموزش اتصال انتخاب کنید:"
    )
    await message.answer(text, reply_markup=get_setup_inline_keyboard(active_key), parse_mode="Markdown")

@router.callback_query(F.data == "open_setup")
async def cb_open_setup(callback: CallbackQuery):
    await show_setup_guide(callback.message)
    await callback.answer()

@router.callback_query(F.data == "setup_cursor")
async def cb_setup_cursor(callback: CallbackQuery):
    text = (
        "💻 **تنظیم در Cursor و VSCode (افزونه Continue یا Cline):**\n\n"
        "۱. وارد Settings (تنظیمات) برنامه شوید.\n"
        "۲. بخش **Models** یا **OpenAI API** را باز کنید.\n"
        f"۳. در کادر **Base URL** مقدار زیر را قرار دهید:\n`{PUBLIC_BASE_URL}`\n"
        "۴. در کادر **API Key** کلید اختصاصی خود را وارد کنید.\n"
        "۵. در بخش Model Name نام `claude-opus-5.5` را تایپ و ذخیره کنید.\n\n"
        "اکنون ادیتور شما بدون فیلتر و با سرعت بالا به مدل متصل است!"
    )
    await callback.message.answer(text, parse_mode="Markdown")
    await callback.answer()

@router.callback_query(F.data == "setup_chatbox")
async def cb_setup_chatbox(callback: CallbackQuery):
    text = (
        "📱 **تنظیم در نرم‌افزار Chatbox (موبایل و دسکتاپ):**\n\n"
        "۱. برنامه Chatbox را باز کنید و وارد Settings شوید.\n"
        "۲. ارائه‌دهنده را روی **OpenAI API** بگذارید.\n"
        f"۳. در بخش **API Host** مقدار زیر را وارد کنید:\n`{PUBLIC_BASE_URL}`\n"
        "۴. کلید خود را در بخش **API Key** وارد کنید.\n"
        "۵. مدل را روی `claude-opus-5.5` تنظیم کنید."
    )
    await callback.message.answer(text, parse_mode="Markdown")
    await callback.answer()
