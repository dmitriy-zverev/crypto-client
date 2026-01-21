from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy import desc, select
from sqlalchemy.orm import Session

from crypto_client.api.deps import get_db
from crypto_client.api.schemas import LatestPriceOut, PriceTickOut
from crypto_client.db.models import PriceTick

router = APIRouter()


def normalize_ticker(ticker: str) -> str:
    return ticker.strip().lower()


@router.get("/prices", response_model=list[PriceTickOut])
def get_all_prices(
    ticker: str = Query(..., description="Ticker, e.g. btc_usd or eth_usd"),
    limit: int = Query(1000, ge=1, le=10000),
    offset: int = Query(0, ge=0),
    db: Session = Depends(get_db),
) -> list[PriceTick]:
    t = normalize_ticker(ticker)

    stmt = select(PriceTick).where(PriceTick.ticker == t).order_by(desc(PriceTick.ts)).limit(limit).offset(offset)
    rows = db.execute(stmt).scalars().all()
    return rows


@router.get("/prices/latest", response_model=LatestPriceOut)
def get_latest_price(
    ticker: str = Query(..., description="Ticker, e.g. btc_usd or eth_usd"),
    db: Session = Depends(get_db),
) -> PriceTick:
    t = normalize_ticker(ticker)

    stmt = select(PriceTick).where(PriceTick.ticker == t).order_by(desc(PriceTick.ts)).limit(1)
    row = db.execute(stmt).scalars().first()
    if row is None:
        raise HTTPException(status_code=404, detail="No data for this ticker yet")
    return row


@router.get("/prices/range", response_model=list[PriceTickOut])
def get_prices_by_date(
    ticker: str = Query(..., description="Ticker, e.g. btc_usd or eth_usd"),
    from_ts: int | None = Query(None, description="Unix timestamp (inclusive)"),
    to_ts: int | None = Query(None, description="Unix timestamp (inclusive)"),
    limit: int = Query(1000, ge=1, le=10000),
    offset: int = Query(0, ge=0),
    db: Session = Depends(get_db),
) -> list[PriceTick]:
    t = normalize_ticker(ticker)

    stmt = select(PriceTick).where(PriceTick.ticker == t)

    if from_ts is not None:
        stmt = stmt.where(PriceTick.ts >= from_ts)
    if to_ts is not None:
        stmt = stmt.where(PriceTick.ts <= to_ts)

    stmt = stmt.order_by(desc(PriceTick.ts)).limit(limit).offset(offset)
    rows = db.execute(stmt).scalars().all()
    return rows
