# cortex-cc

> On-Prem AI Copilot for Contact Centers — powered by MCP + Ollama

Mitel CX 2.0 ships AI via Talkative AI, a cloud dependency. Enterprises in healthcare, banking, and government **cannot send call recordings to the cloud**. `cortex-cc` fills that gap.

## What It Does

- **MCP Server** — exposes contact center state as tools any local LLM can call
- **Ollama Integration** — runs Llama 3.1:8b locally, zero data leaves the server
- **Real-Time Transcription** — Whisper-powered, streamed per call
- **Sentiment Analysis** — live customer mood per call, alerts on drops
- **Supervisor Copilot** — natural language chat: *"Which agents are struggling?"*
- **Proactive Alerts** — LLM monitors queue health and fires alerts autonomously
- **Post-Call Summaries** — structured JSON summaries auto-generated after each call

## Architecture

```
Browser (Supervisor UI)
    │  WebSocket + REST
    ▼
Go API Server ──────────────── SQLite
    │
    ├── Call Simulation Engine (goroutines)
    │       └── fake SIP events, queues, agents, transcripts
    │
    ├── MCP Server (mark3labs/mcp-go)
    │       └── tools: queue, agents, transcripts, route, flag, summarize, sentiment
    │
    ├── Ollama Client (Llama 3.1:8b — local)
    │
    ├── Whisper Transcription (on-prem)
    │
    └── HuggingFace Sentiment (local pipeline)
```

## Stack

| Layer | Tech |
|---|---|
| Backend | Go 1.22 |
| MCP | `mark3labs/mcp-go` |
| LLM | Ollama + Llama 3.1:8b |
| Transcription | Whisper (openai/whisper) |
| Sentiment | HuggingFace `transformers` |
| Storage | SQLite (`modernc.org/sqlite`) |
| Real-time | WebSocket (`gorilla/websocket`) |
| Frontend | HTML + TailwindCSS |
| Deploy | Docker + Docker Compose |

## Quick Start

```bash
# 1. Install Ollama and pull the model
ollama pull llama3.1:8b

# 2. Start everything
docker-compose up

# 3. Open the supervisor UI
open http://localhost:8080
```

## Why This Matters for Mitel

Mitel CX 2.0 embeds Talkative AI — a closed, cloud-only system. A hospital cannot use it. `cortex-cc` is the open, pluggable, on-prem alternative — swap Llama for any Ollama-supported model, extend tools via MCP, deploy behind your own firewall.
