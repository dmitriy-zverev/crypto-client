.PHONY: build-run run server lint test test-cov

build-run:
	docker compose up --build

run:
	docker compose up

server:
	fastapi dev src/crypto_client/api/api.py

lint:
	pre-commit run --all-files

test:
	pytest tests

test-cov:
	pytest tests --cov=. --cov-config=tests/.coveragerc --cov-report term

migrate:
	docker compose run --rm api alembic upgrade head

makemigrations:
	docker compose run --rm api alembic revision --autogenerate -m "$(m)"

db-up:
	docker compose up -d db

psql:
	docker compose exec db psql -U app -d crypto
