from flask import Flask, render_template
from config import Config
from models import db
from flask_migrate import Migrate
from api.agent_routes import agent_bp
from api.script_routes import script_bp
from api.result_routes import result_bp
from api.health_routes import health_bp
from api.logs_routes import logs_bp
from api.config_routes import config_bp

# Initialize Migrate after db
migrate = Migrate()

def create_app():
    app = Flask(__name__, 
                template_folder='dashboard/templates',
                static_folder='dashboard/static')
    app.config.from_object(Config)

    # Initialize extensions
    db.init_app(app)
    migrate.init_app(app, db)   # <-- This enables 'flask db' commands

    # Register blueprints
    app.register_blueprint(config_bp)
    app.register_blueprint(agent_bp, url_prefix="/api/v1/agent")
    app.register_blueprint(script_bp, url_prefix="/api/v1/script")
    app.register_blueprint(result_bp, url_prefix="/api/v1/result")
    app.register_blueprint(logs_bp, url_prefix="/api/v1/logs")
    app.register_blueprint(health_bp)

    @app.route("/")
    def index():
        return render_template("index.html")

    @app.route("/scripts")
    def scripts():
        return render_template("scripts.html")

    @app.route("/results")
    def results():
        return render_template("results.html")

    @app.route("/realtime")
    def realtime():
        return render_template("realtime.html")

    return app

# Create application instance
app = create_app()

if __name__ == "__main__":
    with app.app_context():
        # This will create all tables if they don't exist
        db.create_all()
        print("✅ Database tables created/verified.")
    app.run(host="0.0.0.0", port=5000, debug=True)