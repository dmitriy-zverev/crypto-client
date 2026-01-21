from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from crypto_client.core.config import settings

engine = create_engine(settings.database_url, pool_pre_ping=True)
SessionLocal = sessionmaker(bind=engine, autoflush=False, autocommit=False)
