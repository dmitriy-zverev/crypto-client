## Crypto Client

A service that periodically collects index prices from Deribit and exposes an external API to read the stored data.

The project is designed as a minimal core that can be extended over time. The current scope covers fetching `btc_usd` and `eth_usd` once per minute and providing the required API endpoints. The architecture is intentionally kept modular so that adding new tickers, additional data sources, aggregations, alerts, or observability features does not require rewriting the core.

### Features

Data collection.

A background worker fetches Deribit index prices for `btc_usd` and `eth_usd` once per minute and stores each sample in PostgreSQL with the ticker, the price, and the event time as a UNIX timestamp.

External API.

A FastAPI application provides read-only endpoints to query the stored data. All endpoints use the `GET` method and require the `ticker` query parameter.

Supported queries include retrieving all stored records for a ticker, retrieving the latest price for a ticker, and retrieving prices for a ticker within a specified time range.

### Tech stack

Python 3.12.

FastAPI + Uvicorn.

Celery + Redis (broker).

PostgreSQL.

SQLAlchemy + Alembic.

Docker Compose for local development.

### Design decisions

Separation of concerns and process boundaries.

The solution is split into two independent processes: a data collector (Celery) and an HTTP API (FastAPI). The collector is responsible for fetching external data and persisting it to PostgreSQL. The API is responsible for reading from PostgreSQL and serving query results. The two processes communicate through the database, not via direct HTTP calls, which reduces coupling and improves reliability.

Scheduling with Celery Beat.

Periodic execution is implemented with Celery. Celery Beat schedules tasks at a fixed interval, and Celery workers execute the scheduled tasks. This separation provides predictable behavior and makes it easier to operate and troubleshoot.

Explicit Celery task naming.

Celery tasks use explicit names to keep scheduling stable and independent of module paths and function names. This avoids common issues where beat publishes a task name that does not match what the worker has registered.

Timestamp and numeric precision.

Event time is stored as a UNIX timestamp in seconds to keep filtering simple, unambiguous, and timezone-independent. Price values are stored using a fixed-precision numeric type in PostgreSQL to avoid floating-point rounding issues.

Configuration and avoiding global state.

All configuration is provided via environment variables. External clients and database connections are created through explicit factories or dependency wiring rather than global module-level objects, which improves testability and reduces import-time side effects.

### Future roadmap

The codebase is structured to support straightforward extensions such as adding more tickers, supporting additional exchanges, introducing data retention policies, adding aggregation endpoints (OHLC, averages), implementing alerting, adding metrics and tracing, and switching selected components to an async stack where it provides measurable value.
