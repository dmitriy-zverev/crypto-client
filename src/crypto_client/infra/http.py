from __future__ import annotations

from functools import lru_cache

import requests
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry

from crypto_client.core.config import settings


@lru_cache(maxsize=settings.lru_maxsize)
def get_session() -> requests.Session:
    s = requests.Session()

    retry = Retry(
        total=0,
        connect=0,
        read=0,
        status=0,
        redirect=0,
        backoff_factor=0,
        raise_on_status=False,
        respect_retry_after_header=False,
    )

    adapter = HTTPAdapter(
        max_retries=retry,
        pool_connections=settings.pool_connections,
        pool_maxsize=settings.pool_maxsize,
    )
    s.mount("https://", adapter)
    s.mount("http://", adapter)

    return s
