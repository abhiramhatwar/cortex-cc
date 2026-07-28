FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY service.py .

# Pre-download the model at build time so the container starts instantly
RUN python -c "from transformers import pipeline; pipeline('sentiment-analysis', model='distilbert-base-uncased-finetuned-sst-2-english')"

ENV SENTIMENT_MODEL=distilbert-base-uncased-finetuned-sst-2-english
ENV SENTIMENT_DEVICE=-1
ENV PORT=5001

EXPOSE 5001

CMD ["python", "service.py"]
