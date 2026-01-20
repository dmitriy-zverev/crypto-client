from fastapi import FastAPI

from crypto_client.core.config import settings

app = FastAPI(title=settings.app_name)


@app.get("/health")
def health() -> dict:
    return {"status": "ok", "env": settings.env}
