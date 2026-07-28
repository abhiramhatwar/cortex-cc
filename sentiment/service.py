"""
cortex-cc Sentiment Analysis Service

On-prem NLP microservice using HuggingFace Transformers.
Scores contact center conversation text from -1.0 (very negative)
to +1.0 (very positive). Model runs entirely on local hardware.
"""

import os
import logging
import time
from typing import List

from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel
from transformers import pipeline
import uvicorn

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger("sentiment-service")

MODEL_NAME = os.getenv("SENTIMENT_MODEL", "distilbert-base-uncased-finetuned-sst-2-english")
DEVICE = int(os.getenv("SENTIMENT_DEVICE", "-1"))  # -1 = CPU, 0 = first GPU

log.info(f"Loading sentiment model '{MODEL_NAME}' ...")
classifier = pipeline("sentiment-analysis", model=MODEL_NAME, device=DEVICE)
log.info("Sentiment model ready.")

app = FastAPI(title="cortex-cc Sentiment Service", version="1.0.0")


class AnalyzeRequest(BaseModel):
    text: str


class BatchAnalyzeRequest(BaseModel):
    texts: List[str]


def to_score(label: str, confidence: float) -> float:
    """Map HuggingFace label+confidence to [-1, 1] float."""
    score = round(confidence, 4)
    return score if label.upper() == "POSITIVE" else -score


@app.get("/health")
def health():
    return {"status": "ok", "model": MODEL_NAME}


@app.post("/analyze")
def analyze(req: AnalyzeRequest):
    """
    Score a single text string.

    Returns:
    - score: float in [-1.0, 1.0] — negative = bad, positive = good
    - label: "positive" | "negative" | "neutral"
    - confidence: raw model confidence (0–1)
    """
    if not req.text.strip():
        raise HTTPException(status_code=400, detail="text must not be empty")

    t0 = time.time()
    result = classifier(req.text[:512])[0]  # truncate to model max
    elapsed = round(time.time() - t0, 3)

    score = to_score(result["label"], result["score"])
    label = "positive" if score > 0.15 else ("negative" if score < -0.15 else "neutral")

    return JSONResponse({
        "score": score,
        "label": label,
        "confidence": round(result["score"], 4),
        "elapsed": elapsed,
    })


@app.post("/analyze/batch")
def analyze_batch(req: BatchAnalyzeRequest):
    """
    Score a list of texts in one call. More efficient than N single requests.
    Returns a list of {score, label, confidence} in the same order.
    """
    if not req.texts:
        raise HTTPException(status_code=400, detail="texts list must not be empty")
    if len(req.texts) > 64:
        raise HTTPException(status_code=400, detail="max 64 texts per batch")

    truncated = [t[:512] for t in req.texts]
    t0 = time.time()
    results = classifier(truncated)
    elapsed = round(time.time() - t0, 3)

    out = []
    for r in results:
        score = to_score(r["label"], r["score"])
        out.append({
            "score": score,
            "label": "positive" if score > 0.15 else ("negative" if score < -0.15 else "neutral"),
            "confidence": round(r["score"], 4),
        })

    return JSONResponse({"results": out, "elapsed": elapsed})


if __name__ == "__main__":
    port = int(os.getenv("PORT", "5001"))
    uvicorn.run(app, host="0.0.0.0", port=port, log_level="info")
