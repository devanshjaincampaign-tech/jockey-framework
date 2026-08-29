import os
from dotenv import load_dotenv

load_dotenv()

class Config:
    SECRET_KEY= os.getenv("SECRET_KEY","jocky_sih_2024_dev")
    SQLALCHEMY_DATABASE_URI = os.getenv("DATABASE_URL", "sqlite:////tmp/jocky.db")
    SQLALCHEMY_TRACK_MODIFICATIONS = False
    JWT_EXPIRATION = 3600  # seconds

    SQLALCHEMY_ENGINE_OPTIONS = {
        "pool_pre_ping": True,       # Check connection before using
        "pool_recycle": 300,         # Recycle connections every 5 minutes
        "pool_size": 10,
        "max_overflow": 20
    }