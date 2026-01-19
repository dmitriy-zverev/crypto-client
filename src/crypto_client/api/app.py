from fastapi import FastAPI

app = FastAPI(title="crypto-client")


@app.get("/health")
def ping() -> dict:
    return {"status": "ok"}
