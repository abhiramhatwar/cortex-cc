FROM python:3.11-slim

# ffmpeg is required by openai-whisper for audio decoding
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY service.py .

ENV WHISPER_MODEL=base
ENV WHISPER_DEVICE=cpu
ENV PORT=8001

EXPOSE 8001

CMD ["python", "service.py"]
