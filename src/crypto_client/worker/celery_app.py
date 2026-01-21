from celery import Celery
from celery.schedules import crontab

from crypto_client.core.config import settings
from crypto_client.core.logging import setup_logging


def make_celery_app() -> Celery:
    setup_logging()

    app = Celery(
        "crypto_client",
        broker=settings.celery_broker_url,
        backend=settings.celery_result_backend,
        include=["crypto_client.worker.tasks"],
    )

    app.conf.beat_schedule = {
        "fetch-indexes-every-minute": {
            "task": "client.fetch_indexes",
            "schedule": crontab(minute="*"),
        },
    }

    return app


app = make_celery_app()
