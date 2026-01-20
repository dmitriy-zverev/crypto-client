.PHONY: build run server lint test test-cov

build:
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
