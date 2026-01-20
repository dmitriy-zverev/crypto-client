from celery import Celery
from celery.schedules import crontab

from crypto_client.core.config import settings


def make_celery_app() -> Celery:
    app = Celery(
        "crypto_client",
        broker=settings.celery_broker_url,
        backend=settings.celery_result_backend,
        include=["crypto_client.worker.tasks"],
    )

    return app


app = make_celery_app()

app.conf.beat_schedule = {
    "fetch-indexes": {
        "task": "client.fetch_indexes",
        "schedule": settings.request_frequency
        if settings.request_frequency > 0
        else crontab(),
    },
}
