import time

from crypto_client.clients.deribit import DeribitClient
from crypto_client.clients.errors import PermanentDeribitError, TransientDeribitError
from crypto_client.core.config import settings

from .celery_app import app


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
    prices: dict[str, float] = {}

    for ticker in settings.tickers:
        try:
            prices[ticker] = client.get_index_price(ticker)
        except PermanentDeribitError:
            raise

    return {"attempt": self.request.retries, "ts": ts, "prices": prices}
