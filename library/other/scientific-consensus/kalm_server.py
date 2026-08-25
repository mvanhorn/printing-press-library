# kalm_server.py
# Embedding server for scientific-consensus CLI
# Supports multiple models via environment variable KALM_MODEL
# Production-ready with Waitress WSGI server and restricted CORS.
#
# Logging:
#   - By default, only POST /embed requests are logged.
#   - Set LOG_LEVEL=DEBUG to log all requests (health, info, embed).

import os
import gc
import logging
from flask import Flask, request, jsonify
from flask_cors import CORS
from sentence_transformers import SentenceTransformer
from waitress import serve

# ----------------------------
# Flask App Configuration
# ----------------------------
app = Flask(__name__)

# Increase max content length to 50 MB to handle large abstracts
app.config['MAX_CONTENT_LENGTH'] = 50 * 1024 * 1024  # 50 MB

# ----------------------------
# CORS Configuration – restrict to trusted origins only
# ----------------------------
ALLOWED_ORIGINS = [
    "http://localhost:3000",
    "http://127.0.0.1:3000",
    "http://localhost:8080",
    "http://127.0.0.1:8080",
]
CORS(app, origins=ALLOWED_ORIGINS)

# ----------------------------
# Logging Configuration
# ----------------------------
# Log level can be overridden with LOG_LEVEL environment variable.
# Default: INFO (only /embed requests are logged)
# DEBUG: all requests (health, info, embed) are logged
LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO").upper()
logging.basicConfig(level=getattr(logging, LOG_LEVEL, logging.INFO))
logger = logging.getLogger(__name__)

# Log only POST /embed requests by default (health/info are silent)
@app.before_request
def log_request_info():
    if request.path == '/embed' and request.method == 'POST':
        logger.info(f"📨 POST /embed from {request.remote_addr}")
    elif logger.isEnabledFor(logging.DEBUG):
        # If DEBUG level is enabled, log all requests
        logger.debug(f"📨 {request.method} {request.path} from {request.remote_addr}")

# ----------------------------
# Model Configuration
# ----------------------------
# Default model: gsarti/biobert-nli – optimized for biomedical texts.
# Override with environment variable: set KALM_MODEL=all-MiniLM-L6-v2
DEFAULT_MODEL = "gsarti/biobert-nli"
MODEL_NAME = os.getenv("KALM_MODEL", DEFAULT_MODEL)
logger.info(f"🧠 Using model: {MODEL_NAME}")

# Global model instance (loaded once)
model = None

def load_model():
    """Load the SentenceTransformer model (lazy initialization)."""
    global model
    if model is None:
        logger.info(f"⏳ Loading model: {MODEL_NAME}...")
        model = SentenceTransformer(MODEL_NAME)
        logger.info("✅ Model loaded.")
    return model

# ----------------------------
# API Routes
# ----------------------------
@app.route('/embed', methods=['POST'])
def embed():
    """
    Generate an embedding for a single text.
    Expects JSON: {"text": "string", "normalize": bool}
    Returns: {"embedding": [float], "dimensions": int, "model": "name"}
    """
    data = request.get_json()
    if not data or 'text' not in data:
        return jsonify({"error": "Missing 'text' field"}), 400

    text = data['text']
    normalize = data.get('normalize', True)

    # Ensure model is loaded
    model = load_model()

    # Generate embedding with safe defaults
    embedding = model.encode(
        text,
        normalize_embeddings=normalize,
        batch_size=1,          # Minimize memory usage
        show_progress_bar=False
    )

    # Trigger garbage collection to free memory after large embeddings
    gc.collect()

    return jsonify({
        "embedding": embedding.tolist(),
        "dimensions": len(embedding),
        "model": MODEL_NAME
    })

@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint for the Go client."""
    return jsonify({
        "status": "ok",
        "model": MODEL_NAME
    })

@app.route('/info', methods=['GET'])
def info():
    """Return detailed information about the current model."""
    return jsonify({
        "model": MODEL_NAME,
        "dimensions": 1024 if "bge" in MODEL_NAME else (384 if "MiniLM" in MODEL_NAME else 768),
        "env": {
            "KALM_MODEL": os.getenv("KALM_MODEL", "not set (using default)")
        }
    })

# ----------------------------
# Main Entry Point – using Waitress for production
# ----------------------------
if __name__ == '__main__':
    # Pre-load the model before starting the server
    load_model()

    # Bind to 127.0.0.1 only to avoid external access
    # Use Waitress instead of Flask's built-in development server
    serve(app, host='127.0.0.1', port=5000, threads=4)