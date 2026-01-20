import time

from requests.exceptions import RequestException

from crypto_client.core.config import settings
from crypto_client.infra.http import get_session

from .celery_app import app


@app.task(
    bind=True,
    name="client.fetch_indexes",
    autoretry_for=(RequestException,),
    retry_backoff=True,
    retry_jitter=True,
    retry_kwargs={"max_retries": settings.http_max_retries},
)
def fetch_indexes(self) -> dict:
    ts = int(time.time())
    prices: dict[str, float] = {}

    session = get_session()

    for ticker in settings.tickers:
        url_path = f"/public/get_index_price?index_name={ticker}"
        resp = session.get(
            settings.deribit_rpc_url + url_path,
            timeout=settings.http_timeout_seconds,
        )
        resp.raise_for_status()
        data = resp.json()
        prices[ticker] = float(data["result"]["index_price"])

    return {"attempt": self.request.retries, "ts": ts, "prices": prices}
