import time
from decimal import Decimal

from crypto_client.clients.deribit import DeribitClient
from crypto_client.clients.errors import TransientDeribitError
from crypto_client.core.config import settings
from crypto_client.db.engine import SessionLocal
from crypto_client.db.models import PriceTick
from crypto_client.worker.celery_app import app


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
    client = DeribitClient()

    rows: list[PriceTick] = []
    for ticker in settings.tickers:
        price = client.get_index_price(ticker)
        rows.append(
            PriceTick(
                ticker=ticker,
                ts=ts,
                price=Decimal(str(price)),
            )
        )

    db = SessionLocal()
    try:
        db.add_all(rows)
        db.commit()
    except Exception:
        db.rollback()
        raise
    finally:
        db.close()

    return {"attempt": self.request.retries, "ts": ts, "saved": len(rows)}
