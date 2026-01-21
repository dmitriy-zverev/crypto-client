# Deribit Crypto Client

*A small backend service that periodically fetches index prices from the Deribit exchange and exposes stored data via a FastAPI HTTP API.*

The project is built as a production-style mini service. It includes a background scheduler (Celery Beat), workers (Celery Worker), an HTTP API (FastAPI), PostgreSQL for storage, and Redis as a Celery broker/result backend.

---

## 1. Features

**Deribit client**. Fetches index prices for configured tickers (e.g. `btc_usd`, `eth_usd`) from the Deribit public endpoint `get_index_price`.

**Periodic ingestion**. A Celery Beat schedule triggers a task every minute. The task fetches prices for all configured tickers and writes them to PostgreSQL with `ticker`, `price`, and `ts` (UNIX timestamp).

**HTTP API (FastAPI)**. All endpoints are GET and require the `ticker` query parameter.

- `GET /prices`
  Returns stored rows for a ticker.
- `GET /prices/latest`
  Returns the most recent price for a ticker.
- `GET /prices/range`
  Returns prices for a ticker filtered by UNIX timestamps (`from_ts`, `to_ts`).

**Health checks**.
- `GET /health` returns 200 only if the database connection is available, otherwise 503.

**Logging**. Unified application logging for both API and worker processes, with request logging in the API and structured error logging in Celery tasks.

---

## 2. Requirements

- Docker + Docker Compose

Optional for local development.
- Python 3.12
- uv (or any virtualenv tool)

---

## 3. Quick start

Clone the repository, then run the stack.

```bash
docker compose up -d --build
```

Apply database migrations.

```bash
docker compose run --rm api alembic upgrade head
```

Verify that the API is healthy.

```bash
curl -i http://localhost:8000/health
```

Wait 1–2 minutes for the scheduled task to populate data, then query the API.

```bash
curl "http://localhost:8000/prices/latest?ticker=btc_usd"
curl "http://localhost:8000/prices/latest?ticker=eth_usd"
```

You can also query the DB directly.

```bash
docker compose exec db psql -U app -d crypto -c "select ticker, ts, price from price_ticks order by ts desc limit 10;"
```

---

## 4. Configuration

The service is configured via environment variables. See `.env.example` for a complete list.

Create a local `.env` file.

```bash
cp .env.example .env
```

Key variables.

* `TICKERS`
  Comma-separated list of tickers to fetch.
  Example: `btc_usd,eth_usd`

* `DERIBIT_RPC_URL`
  Base URL for Deribit API.
  Example: `https://www.deribit.com/api/v2`

* `HTTP_TIMEOUT_SECONDS`
  Requests timeout.

* `HTTP_MAX_RETRIES`
  Max retries for transient HTTP errors.

* `LOG_LEVEL`
  Logging level.
  Example: `INFO` or `DEBUG`

Database.

* `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_HOST`, `POSTGRES_PORT`
* `DATABASE_URL` (if you use a single SQLAlchemy URL instead of parts)

Celery.

* `CELERY_BROKER_URL`
* `CELERY_RESULT_BACKEND`

---

## 5. API

All endpoints are GET.
All endpoints require `ticker` as a query parameter.

### 5.1 Get all saved data for a ticker

```bash
curl "http://localhost:8000/prices?ticker=btc_usd"
```

Optional pagination parameters.

* `limit` (default 1000)
* `offset` (default 0)

```bash
curl "http://localhost:8000/prices?ticker=btc_usd&limit=50&offset=0"
```

### 5.2 Get the latest price for a ticker

```bash
curl "http://localhost:8000/prices/latest?ticker=btc_usd"
```

If there is no data for the ticker yet, the endpoint returns 404.

### 5.3 Get prices filtered by UNIX timestamps

Parameters.

* `from_ts` inclusive, optional
* `to_ts` inclusive, optional

```bash
curl "http://localhost:8000/prices/range?ticker=btc_usd&from_ts=1700000000&to_ts=1800000000"
```

---

## 6. Running tests

Unit tests are designed to run in the API container.

```bash
docker compose run --rm api pytest -q
```

**Notes**. The API tests override the DB dependency with an in-memory SQLite database, so tests are fast and do not depend on Postgres availability. Postgres-specific behavior can be covered by integration tests as a future improvement.

---

## 7. Development commands

Build and start services.

```bash
docker compose up -d --build
```

View logs.

```bash
docker compose logs -f
docker compose logs -f api
docker compose logs -f worker
docker compose logs -f beat
```

Apply migrations.

```bash
docker compose run --rm api alembic upgrade head
```

Create a new migration (local development).
Run this on your machine, commit generated migration files.

```bash
alembic revision --autogenerate -m "message"
```

---

## 8. Database migrations

This repository includes Alembic migrations and they must be committed to Git.

Why.

* Migrations are part of the deployable artifact.
* They allow deterministic schema changes across environments.
* A fresh clone should be able to reach the correct schema with `alembic upgrade head`.

Typical workflow.

* Change SQLAlchemy models.
* Generate a migration file.
* Review the generated migration.
* Commit the migration under `alembic/versions`.
* Apply migrations in environments with `alembic upgrade head`.

---

## 9. Design decisions

**Celery + Redis**. Celery is used for periodic background execution, separating ingestion from the HTTP API. Redis is used as a broker/result backend because it is simple to run in Docker Compose and is sufficient for this workload. RabbitMQ would also work and may be preferred for more complex routing patterns or heavier messaging workloads.

**One task per tick**. The schedule triggers one periodic task that fetches all configured tickers and writes them in a single DB transaction. This reduces scheduling complexity and makes ingestion behavior predictable.

**UNIX timestamps**. Timestamps are stored as integer UNIX seconds to avoid timezone ambiguity and to keep range filtering simple.

**Error handling and retries**. Deribit requests are executed with a session and retry logic. Transient failures (timeouts, connection errors, malformed JSON responses, 5xx) are retried to improve resilience. Non-transient failures are surfaced early. Tasks use consistent logging so failures can be diagnosed by ticker and attempt number.

**API is read-only**. The API provides read access to stored ticks, supporting:

* full list for a ticker,
* latest value,
* range filtering by time.
  This matches the problem statement and keeps the service responsibilities clear.

**Unified logging**. A shared `setup_logging()` is used across API and worker processes to ensure consistent formatting and levels controlled by config. The API logs request metadata (method, path, status, duration). Workers log task start, success, and exceptions with context.

---

## 10. Project structure

```text
.
├── alembic/
│   ├── versions/
│   └── env.py
├── src/
│   └── crypto_client/
│       ├── api/
│       │   ├── main.py
│       │   ├── deps.py
│       │   ├── routes.py
│       │   └── schemas.py
│       ├── clients
│       │   ├── deribit.py
│       │   └── errors.py
│       ├── core/
│       │   ├── config.py
│       │   └── logging.py
│       ├── db/
│       │   ├── base.py
│       │   ├── engine.py
│       │   └── models.py
│       └── worker/
│           ├── celery_app.py
│           └── tasks.py
├── tests/
├── docker-compose.yml
├── Dockerfile
├── alembic.ini
└── README.md
```

---

## 11. Future improvements

* Replace requests with aiohttp and make ingestion async-friendly.
* Add integration tests running against Postgres.
* Improve observability (structured JSON logs, request IDs).
* Run containers as a non-root user to remove Celery security warnings.
* Add retention/cleanup policy for old ticks or partition by time.
