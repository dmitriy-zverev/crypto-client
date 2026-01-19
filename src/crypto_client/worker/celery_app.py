import os

from celery import Celery


def make_celery_app() -> Celery:
    broker_url = os.getenv("CELERY_BROKER_URL", "redis://redis:6379/0")
    backend_url = os.getenv("CELERY_RESULT_BACKEND", broker_url)

    app = Celery(
        "crypto_client",
        broker=broker_url,
        backend=backend_url,
        include=["crypto_client.worker.tasks"],
    )

    return app


app = make_celery_app()

app.conf.beat_schedule = {
    "health-every-60-seconds": {
        "task": "debug.health",
        "schedule": 60.0,
    },
}
