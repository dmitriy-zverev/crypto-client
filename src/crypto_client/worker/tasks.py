import logging
import time
from decimal import Decimal

from crypto_client.clients.deribit import DeribitClient
from crypto_client.clients.errors import TransientDeribitError
from crypto_client.core.config import settings
from crypto_client.db.engine import SessionLocal
from crypto_client.db.models import PriceTick
from crypto_client.worker.celery_app import app

logger = logging.getLogger(__name__)


@app.task(
    bind=True,
    name="client.fetch_indexes",
    autoretry_for=(TransientDeribitError,),
    retry_backoff=True,
    retry_jitter=True,
    retry_kwargs={"max_retries": settings.http_max_retries},
)
def fetch_indexes(self) -> dict:
    ts = int(time.time())
    attempt = self.request.retries

    logger.info(
        "fetch_indexes started ts=%s attempt=%s tickers=%s",
        ts,
        attempt,
        settings.tickers,
    )

    client = DeribitClient()
    rows: list[PriceTick] = []

    for ticker in settings.tickers:
        try:
            price = client.get_index_price(ticker)
            rows.append(PriceTick(ticker=ticker, ts=ts, price=Decimal(str(price))))
        except Exception:
            logger.exception(
                "fetch_indexes failed ticker=%s ts=%s attempt=%s", ticker, ts, attempt
            )
            raise

    db = SessionLocal()
    try:
        db.add_all(rows)
        db.commit()
    except Exception:
        logger.exception(
            "db commit failed ts=%s attempt=%s rows=%s", ts, attempt, len(rows)
        )
        db.rollback()
        raise
    finally:
        db.close()

    logger.info(
        "fetch_indexes succeeded ts=%s attempt=%s saved=%s", ts, attempt, len(rows)
    )
    return {"attempt": self.request.retries, "ts": ts, "saved": len(rows)}
