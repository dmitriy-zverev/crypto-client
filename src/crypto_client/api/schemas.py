from pydantic import BaseModel


class PriceTickOut(BaseModel):
    ticker: str
    ts: int
    price: float

    model_config = {"from_attributes": True}


class LatestPriceOut(BaseModel):
    ticker: str
    ts: int
    price: float
