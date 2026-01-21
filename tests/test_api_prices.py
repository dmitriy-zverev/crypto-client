import time

from crypto_client.db.models import PriceTick


def seed(db, ticker: str, prices: list[float], start_ts: int) -> None:
    ts = start_ts
    for p in prices:
        db.add(PriceTick(ticker=ticker, ts=ts, price=p))
        ts += 60
    db.commit()


def test_latest_returns_404_when_no_data(client):
    resp = client.get("/prices/latest", params={"ticker": "btc_usd"})
    assert resp.status_code == 404


def test_latest_returns_most_recent(client, db):
    now = int(time.time())
    seed(db, "btc_usd", [100.0, 101.5, 99.9], now)

    resp = client.get("/prices/latest", params={"ticker": "btc_usd"})
    assert resp.status_code == 200
    data = resp.json()
    assert data["ticker"] == "btc_usd"
    assert float(data["price"]) == 99.9
    assert data["ts"] == now + 120


def test_range_filters_by_ts(client, db):
    base = 1_700_000_000
    seed(db, "eth_usd", [10.0, 11.0, 12.0, 13.0], base)

    resp = client.get(
        "/prices/range",
        params={"ticker": "eth_usd", "from_ts": base + 60, "to_ts": base + 120},
    )
    assert resp.status_code == 200
    data = resp.json()
    assert [x["ts"] for x in data] == [base + 120, base + 60]
