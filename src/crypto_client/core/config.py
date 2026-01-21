from __future__ import annotations

from typing import Literal

from pydantic import Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """
    Single source of truth for configuration.

    Reads values from environment variables (and optional .env file),
    applies defaults, and validates types early at startup.
    """

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="ignore",
    )

    # Common
    app_name: str = Field(default="crypto-client", validation_alias="APP_NAME")
    env: Literal["dev", "test", "prod"] = Field(default="dev", validation_alias="ENV")
    log_level: str = Field(default="INFO", validation_alias="LOG_LEVEL")

    # Database
    database_url: str = Field(
        default="postgresql+psycopg2://app:app@db:5432/crypto",
        validation_alias="DATABASE_URL",
        description="SQLAlchemy database URL",
    )

    # Celery
    celery_broker_url: str = Field(
        default="redis://redis:6379/0",
        validation_alias="CELERY_BROKER_URL",
    )
    celery_result_backend: str = Field(
        default="redis://redis:6379/0",
        validation_alias="CELERY_RESULT_BACKEND",
    )

    # Deribit
    deribit_rpc_url: str = Field(
        default="https://www.deribit.com/api/v2",
        validation_alias="DERIBIT_RPC_URL",
        description="Deribit JSON-RPC endpoint",
    )
    tickers: list[str] = Field(
        default_factory=lambda: ["btc_usd", "eth_usd"],
        validation_alias="TICKERS",
        description="Comma-separated list in env, e.g. 'btc_usd,eth_usd'",
    )

    # HTTP client behavior
    http_timeout_seconds: float = Field(
        default=5.0, validation_alias="HTTP_TIMEOUT_SECONDS"
    )
    http_max_retries: int = Field(default=3, validation_alias="HTTP_MAX_RETRIES")
    http_backoff_seconds: float = Field(
        default=0.2, validation_alias="HTTP_BACKOFF_SECONDS"
    )
    pool_connections: int = Field(default=10, validation_alias="POOL_CONNECTIONS")
    pool_maxsize: int = Field(default=10, validation_alias="POOL_MAXSIZE")
    lru_maxsize: int = Field(default=1, validation_alias="LRU_MAXSIZE")

    @field_validator("tickers", mode="before")
    @classmethod
    def parse_tickers(cls, v):
        """
        Allow:
        - env: TICKERS="btc_usd,eth_usd"
        - python: ["btc_usd", "eth_usd"]
        """
        if v is None or v == "":
            return ["btc_usd", "eth_usd"]
        if isinstance(v, str):
            items = [x.strip().lower() for x in v.split(",")]
            items = [x for x in items if x]
            return items
        if isinstance(v, list):
            items = [str(x).strip().lower() for x in v]
            items = [x for x in items if x]
            return items
        return v

    @field_validator("tickers")
    @classmethod
    def tickers_not_empty(cls, v: list[str]) -> list[str]:
        if not v:
            raise ValueError("TICKERS must contain at least one ticker")
        return v

    @field_validator("http_timeout_seconds", "http_backoff_seconds")
    @classmethod
    def positive_floats(cls, v: float) -> float:
        if v <= 0:
            raise ValueError("Timeout/backoff must be > 0")
        return v

    @field_validator("http_max_retries")
    @classmethod
    def non_negative_retries(cls, v: int) -> int:
        if v < 0:
            raise ValueError("HTTP_MAX_RETRIES must be >= 0")
        return v

    @field_validator("request_frequency")
    @classmethod
    def non_negative_request_frequency(cls, v: int) -> int:
        if v < 0:
            raise ValueError("HTTP_MAX_RETRIES must be >= 0")
        return v


settings = Settings()
