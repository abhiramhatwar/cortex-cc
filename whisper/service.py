"""
cortex-cc Whisper Transcription Service

On-prem speech-to-text microservice using OpenAI Whisper (open-source model).
Accepts audio file uploads, returns structured transcripts with timestamps.
Zero cloud dependency — model runs entirely on local hardware.
"""

import os
import tempfile
import time
import logging
from pathlib import Path

import whisper
from fastapi import FastAPI, File, UploadFile, HTTPException, Form
from fastapi.responses import JSONResponse
import uvicorn

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger("whisper-service")

MODEL_SIZE = os.getenv("WHISPER_MODEL", "base")  # tiny | base | small | medium | large
DEVICE = os.getenv("WHISPER_DEVICE", "cpu")       # cpu | cuda

log.info(f"Loading Whisper model '{MODEL_SIZE}' on {DEVICE} ...")
model = whisper.load_model(MODEL_SIZE, device=DEVICE)
log.info("Whisper model ready.")

app = FastAPI(title="cortex-cc Whisper Service", version="1.0.0")


@app.get("/health")
def health():
    return {"status": "ok", "model": MODEL_SIZE, "device": DEVICE}


@app.post("/transcribe")
async def transcribe(
    audio: UploadFile = File(..., description="Audio file (wav, mp3, m4a, ogg, flac)"),
    language: str = Form(default=None, description="ISO 639-1 language code, e.g. 'en'. Auto-detect if omitted."),
):
    """
    Transcribe an uploaded audio file using OpenAI Whisper.

    Returns:
    - text: full transcript as a single string
    - segments: list of {id, start, end, text} with timestamps in seconds
    - language: detected or specified language
    - duration: audio duration in seconds
    - elapsed: transcription time in seconds
    """
    allowed = {".wav", ".mp3", ".m4a", ".ogg", ".flac", ".webm", ".mp4"}
    suffix = Path(audio.filename or "audio.wav").suffix.lower()
    if suffix not in allowed:
        raise HTTPException(status_code=400, detail=f"Unsupported format: {suffix}. Allowed: {allowed}")

    audio_bytes = await audio.read()
    if len(audio_bytes) == 0:
        raise HTTPException(status_code=400, detail="Empty audio file")

    log.info(f"Transcribing '{audio.filename}' ({len(audio_bytes)/1024:.1f} KB) ...")
    t0 = time.time()

    with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as tmp:
        tmp.write(audio_bytes)
        tmp_path = tmp.name

    try:
        options = {}
        if language:
            options["language"] = language

        result = model.transcribe(tmp_path, **options)

        segments = [
            {
                "id": int(seg["id"]),
                "start": round(float(seg["start"]), 2),
                "end": round(float(seg["end"]), 2),
                "text": seg["text"].strip(),
            }
            for seg in result.get("segments", [])
        ]

        elapsed = round(time.time() - t0, 2)
        duration = segments[-1]["end"] if segments else 0.0

        log.info(f"Done in {elapsed}s — {len(segments)} segments, language={result.get('language')}")

        return JSONResponse({
            "text": result["text"].strip(),
            "segments": segments,
            "language": result.get("language", "unknown"),
            "duration": duration,
            "elapsed": elapsed,
        })

    finally:
        os.unlink(tmp_path)


if __name__ == "__main__":
    port = int(os.getenv("PORT", "8001"))
    uvicorn.run(app, host="0.0.0.0", port=port, log_level="info")
