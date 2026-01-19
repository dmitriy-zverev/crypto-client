import requests

from .celery_app import app


@app.task(name="debug.health")
def hello() -> str:
    return requests.get("http://api:8000/health").text
