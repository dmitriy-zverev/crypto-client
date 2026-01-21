from __future__ import annotations

from typing import Any

from requests.exceptions import (
    ConnectionError,
    HTTPError,
    RequestException,
    SSLError,
    Timeout,
)

from crypto_client.clients.errors import PermanentDeribitError, TransientDeribitError
from crypto_client.core.config import settings
from crypto_client.infra.http import get_session


class DeribitClient:
    def __init__(self) -> None:
        self._session = get_session()
        self._base_url = settings.deribit_rpc_url
        self._timeout = settings.http_timeout_seconds

    def get_index_price(self, ticker: str) -> float:
        url_path = f"/public/get_index_price?index_name={ticker}"
        url = self._base_url + url_path

        try:
            resp = self._session.get(url, timeout=self._timeout)
            resp.raise_for_status()
        except (Timeout, ConnectionError, SSLError) as e:
            raise TransientDeribitError(str(e)) from e
        except HTTPError as e:
            status = e.response.status_code if e.response is not None else None

            if status in (429, 500, 502, 503, 504):
                raise TransientDeribitError(f"HTTP {status}") from e

            raise PermanentDeribitError(f"HTTP {status}") from e
        except RequestException as e:
            raise TransientDeribitError(str(e)) from e

        try:
            data: dict[str, Any] = resp.json()
        except ValueError as e:
            raise TransientDeribitError("Invalid JSON response") from e

        try:
            price = data["result"]["index_price"]
        except (TypeError, KeyError) as e:
            raise PermanentDeribitError(f"Unexpected response shape: {data}") from e

        try:
            return float(price)
        except (TypeError, ValueError) as e:
            raise PermanentDeribitError(f"Non-numeric index_price: {price!r}") from e
