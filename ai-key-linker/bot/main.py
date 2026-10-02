import asyncio
import logging
from aiogram import Bot, Dispatcher
from aiogram.enums import ParseMode
from aiogram.client.default import DefaultBotProperties

from config import BOT_TOKEN
from services.bot_db import init_bot_db
from tasks.scheduler import notification_worker

from handlers.start import router as start_router
from handlers.status import router as status_router
from handlers.plans import router as plans_router
from handlers.referral import router as referral_router
from handlers.redeem import router as redeem_router
from handlers.setup_guide import router as setup_router
from handlers.admin import router as admin_router

logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")

async def main():
    logging.info("Initializing Bot Database...")
    init_bot_db()

    bot = Bot(token=BOT_TOKEN, default=DefaultBotProperties(parse_mode=ParseMode.MARKDOWN))
    dp = Dispatcher()

    # Register Routers
    dp.include_router(admin_router)
    dp.include_router(start_router)
    dp.include_router(status_router)
    dp.include_router(plans_router)
    dp.include_router(referral_router)
    dp.include_router(redeem_router)
    dp.include_router(setup_router)

    # Start background task
    asyncio.create_task(notification_worker(bot))

    logging.info("Telegram Bot is polling for updates...")
    await dp.start_polling(bot, drop_pending_updates=True)

if __name__ == "__main__":
    asyncio.run(main())
