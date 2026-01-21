import logging
import time

from fastapi import Depends, FastAPI, HTTPException, Request
from sqlalchemy import text

from crypto_client.api.deps import get_db
from crypto_client.api.routes import router
from crypto_client.core.logging import setup_logging

logger = logging.getLogger(__name__)


def create_app() -> FastAPI:
    setup_logging()

    app = FastAPI(title="crypto-client")
    app.include_router(router)

    @app.middleware("http")
    async def log_requests(request: Request, call_next):
        start = time.time()
        response = await call_next(request)
        duration_ms = int((time.time() - start) * 1000)

        logger.info(
            "http request method=%s path=%s status=%s duration_ms=%s",
            request.method,
            request.url.path,
            response.status_code,
            duration_ms,
        )
        return response

    @app.get("/health")
    def health(db=Depends(get_db)) -> dict:
        try:
            db.execute(text("SELECT 1"))
        except Exception:
            raise HTTPException(status_code=503, detail="db unavailable")
        return {"status": "ok"}

    return app


app = create_app()
