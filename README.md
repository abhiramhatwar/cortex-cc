# cortex-cc

**On-Prem AI Copilot for Contact Centers — powered by MCP + Ollama**

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://golang.org)
[![MCP](https://img.shields.io/badge/MCP-Model%20Context%20Protocol-blueviolet)](https://modelcontextprotocol.io)
[![Ollama](https://img.shields.io/badge/LLM-Ollama%20%2B%20Llama%203.1-orange)](https://ollama.com)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

---

## The Problem

Mitel CX 2.0 ships AI features through **Talkative AI** — a cloud-based third-party dependency.

This means a hospital, a bank, or a government call center **cannot use it**. Their call recordings contain patient data, financial data, and classified information. Sending that to the cloud violates HIPAA, PCI-DSS, and dozens of other compliance frameworks.

**The result:** Mitel is losing enterprise deals to RingCentral and Cisco because they can't offer on-prem AI.

---

## The Solution

`cortex-cc` is an **MCP (Model Context Protocol) server** that sits on top of any contact center and exposes its data as tools a local LLM can call and reason over.

No cloud. No external API calls. No data ever leaves your server.

```
Supervisor asks: "Which agents are struggling right now?"

              cortex-cc
                  │
     ┌────────────▼────────────┐
     │      MCP Server (Go)    │
     │                         │
     │  get_agent_states ───►  │◄── Ollama (Llama 3.1 — runs locally)
     │  get_queue_status ───►  │
     │  get_sentiment   ───►   │
     └────────────┬────────────┘
                  │
       "Agent Sarah has handled 14 calls
        today. Her last 3 calls all had
        negative sentiment. She may need
        a break or coaching."
```

The supervisor gets an answer. Zero bytes left the building.

---

## Features

| Feature | Status | Description |
|---|---|---|
| **MCP Server** | ✅ Built | 7 tools exposable to any local LLM via the Model Context Protocol |
| **Natural Language Queries** | ✅ Built | Supervisor chats in plain English — LLM calls the right tools |
| **Agentic Tool-Calling Loop** | ✅ Built | LLM calls multiple tools in sequence (up to 5 rounds) to reason over data |
| **Live Call Queue** | ✅ Built | Real-time WebSocket feed of active calls, wait times, SLA breaches |
| **Agent Monitoring** | ✅ Built | Status, call count, avg handle time, sentiment per agent |
| **Proactive Alerts** | ✅ Built | Monitor polls every 60s, fires alerts + AI advisory without being asked |
| **Post-Call Summaries** | ✅ Built | Structured JSON summary (issue, resolution, follow-up) auto-generated |
| **Supervisor Dashboard** | ✅ Built | Live HTML dashboard — agents, queues, alerts, AI chat panel |
| **Real-Time Transcription** | ✅ Built | Whisper-powered on-prem speech-to-text via `/api/transcribe` |
| **Sentiment Analysis** | ✅ Built | HuggingFace DistilBERT local pipeline with EMA per-call scoring |
| **Agent Assist** | ✅ Built | Real-time Ollama-generated response suggestions during live calls |
| **Fully On-Prem** | ✅ Built | Ollama + local models — nothing leaves your server |

---

## How It Works

### 1. MCP Tools

The MCP server exposes the contact center as a set of callable tools. Any MCP-compatible client (Claude Desktop, Cursor, etc.) can connect and call these directly.

| Tool | What It Does |
|---|---|
| `get_queue_status` | Returns active queue depth, wait times, SLA breach count |
| `get_agent_states` | Returns all agents with status, current call, handle time |
| `get_call_transcript` | Returns live or historical transcript for a call ID |
| `get_sentiment_report` | Returns sentiment scores across all active calls |
| `route_call` | Moves a call to a specific agent |
| `flag_call` | Marks a call for QA review with a reason |
| `summarize_call` | Triggers post-call AI summary generation |

### 2. The Agentic Loop

When a supervisor asks a question, here's what happens under the hood:

```
1. Supervisor types: "Who needs help right now?"
2. Message sent to Ollama (Llama 3.1:8b — local, free)
3. Ollama decides which MCP tools to call
4. cortex-cc executes: get_agent_states + get_sentiment_report
5. Results returned to Ollama
6. Ollama reasons over the live data and responds in plain English
7. Answer appears in supervisor dashboard — zero cloud calls made
```

Up to 5 tool-call rounds per query. Multi-turn conversation history maintained per supervisor session.

### 3. Proactive Anomaly Monitor

cortex-cc runs a background goroutine that polls the queue every 60 seconds and automatically detects:

- **SLA breaches** — warns at 1, critical at 3+
- **Queue overloads** — 5+ calls waiting in any queue
- **Sentiment spikes** — avg sentiment drops below -0.4
- **Agent shortages** — fewer than 10% of agents available

When anomalies are found, two things happen simultaneously:
1. Alert events are broadcast over WebSocket to the supervisor dashboard instantly
2. The LLM generates a 2–3 sentence supervisor advisory with the single most important action to take

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Browser (Supervisor Dashboard)                │
│         AI Chat Panel  │  Live Queues  │  Agent Tiles           │
└──────────┬─────────────┴───────────────┴──────────┬─────────────┘
           │ HTTP REST + POST /api/chat              │ WebSocket
           ▼                                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                          Go HTTP Server                          │
│                                                                  │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────────────┐ │
│  │  REST API    │   │  WebSocket   │   │   Proactive Monitor  │ │
│  │ /api/calls   │   │     Hub      │   │   (60s poll loop)    │ │
│  │ /api/agents  │   │  broadcast   │   │   anomaly detection  │ │
│  │ /api/chat    │   │  to all WS   │   │   + AI advisory      │ │
│  └──────┬───────┘   └──────▲───────┘   └──────────────────────┘ │
│         │                  │                                      │
│  ┌──────▼───────┐   ┌──────┴───────┐   ┌──────────────────────┐ │
│  │  LLM Loop    │   │  Call Engine │   │       SQLite         │ │
│  │  (agentic)   │   │  goroutines  │   │  calls, agents,      │ │
│  │  max 5 rounds│   │  tick / gen  │   │  transcripts,        │ │
│  └──────┬───────┘   └──────────────┘   │  summaries           │ │
│         │                              └──────────────────────┘ │
│  ┌──────▼───────┐   ┌──────────────┐                            │
│  │ Ollama Client│   │  MCP Server  │                            │
│  │ Llama 3.1:8b │   │  (stdio) for │                            │
│  │ (local only) │   │  Claude etc. │                            │
│  └──────────────┘   └──────────────┘                            │
└─────────────────────────────────────────────────────────────────┘
```

---

## Tech Stack

| Layer | Technology | Why |
|---|---|---|
| Backend | **Go 1.25** | Goroutines for high-concurrency call event simulation |
| MCP | **mark3labs/mcp-go** | Go-native MCP server SDK |
| LLM | **Ollama + Llama 3.1:8b** | Free, local, tool-calling capable — no API key |
| Database | **modernc.org/sqlite** | Zero CGO, embedded, single-file database |
| Real-time | **gorilla/websocket** | Live push to supervisor dashboard |
| Frontend | **HTML + Tailwind CSS** | Lightweight, no build step needed |
| Containers | **Docker + Docker Compose** | One-command deploy |

---

## Quick Start

### Prerequisites

- [Go 1.25+](https://golang.org/dl/)
- [Ollama](https://ollama.com) installed and running

### 1. Pull the model

```bash
ollama pull llama3.1:8b
```

### 2. Clone and run

```bash
git clone https://github.com/abhiramhatwar/cortex-cc.git
cd cortex-cc
go run ./cmd/server
```

### 3. Open the supervisor dashboard

```
http://localhost:8080
```

The call simulation engine starts automatically — the dashboard populates with live agents, call queues, and real-time events within seconds.

---

## Usage Examples

### Natural Language Queries (AI Copilot)

```
"Which calls are at risk of breaching SLA?"
→ LLM calls: get_queue_status
→ "3 calls in the Billing queue have been waiting over 4 minutes."

"Route call C-1042 to Sarah."
→ LLM calls: route_call(call_id="C-1042", agent_id="sarah")
→ "Done. Call C-1042 has been transferred to Sarah."

"What are customers complaining about today?"
→ LLM calls: get_sentiment_report → get_call_transcript (for negative calls)
→ "Most complaints today are around invoice delays and account access issues."

"Flag the most negative call for QA."
→ LLM calls: get_sentiment_report → flag_call
→ "Call C-1039 (sentiment -0.82) has been flagged for QA review: distressed customer."
```

### REST API

```bash
# Get all active calls
curl http://localhost:8080/api/calls

# Get all agents and their status
curl http://localhost:8080/api/agents

# Get queue statistics
curl http://localhost:8080/api/queues

# Get live transcript for a call
curl http://localhost:8080/api/calls/C-1042/transcript

# Get post-call summary
curl http://localhost:8080/api/calls/C-1042/summary

# Chat with the AI copilot
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Which agents are struggling right now?"}'

# Reset conversation history
curl -X POST http://localhost:8080/api/chat/reset
```

---

## Why This Matters

| | Mitel CX 2.0 | RingCentral RingSense | cortex-cc |
|---|---|---|---|
| AI Engine | Talkative AI (cloud) | Cloud (proprietary) | Ollama + Llama 3.1 (local) |
| Data leaves server | Yes | Yes | **Never** |
| HIPAA / PCI compliant | No | No | **Yes** |
| MCP support | None | None | **Full** |
| Swap LLM model | No | No | **Yes — any Ollama model** |
| Proactive alerts | Basic | Yes | **Yes + AI advisory** |
| Monthly cost | $65–85/agent | $35–75/agent | **$0** |
| Open source | No | No | **Yes** |

---

## Project Structure

```
cortex-cc/
├── cmd/
│   └── server/
│       └── main.go              # Entry point — wires all components
├── internal/
│   ├── config/                  # Env-based configuration (PORT, DB_PATH, OLLAMA_URL...)
│   ├── models/                  # Domain types: Call, Agent, Transcript, Event
│   ├── store/                   # SQLite persistence — migrations, queries
│   ├── engine/                  # Call simulation engine
│   │   ├── engine.go            # State machine (queued → active → completed)
│   │   ├── generator.go         # Random call generator every 8s
│   │   ├── transcript.go        # Fake transcript lines every 6s
│   │   └── util.go              # Helpers
│   ├── websocket/
│   │   └── hub.go               # WebSocket hub — broadcast to all connected clients
│   ├── mcp/
│   │   ├── tools.go             # 7 tool definitions + handlers (Ollama + MCP compatible)
│   │   └── server.go            # MCP stdio server for Claude Desktop / Cursor
│   ├── llm/
│   │   ├── client.go            # Ollama HTTP client (chat + tool calls)
│   │   └── loop.go              # Agentic loop — multi-turn, max 5 tool rounds
│   ├── monitor/
│   │   └── monitor.go           # Proactive anomaly detection + AI advisories
│   ├── sentiment/
│   │   └── client.go            # Go HTTP client for DistilBERT sentiment service
│   ├── transcriber/
│   │   └── client.go            # Go HTTP client for Whisper STT service
│   ├── assist/
│   │   └── service.go           # Agent assist — rate-limited real-time suggestions
│   └── server/
│       └── server.go            # HTTP handlers: REST API + /api/chat + /api/transcribe
├── web/
│   └── index.html               # Supervisor dashboard (Tailwind, dark theme, WebSocket)
├── whisper/
│   ├── service.py               # FastAPI Whisper STT microservice (port 8001)
│   └── Dockerfile
├── sentiment/
│   ├── service.py               # FastAPI DistilBERT sentiment microservice (port 5001)
│   └── Dockerfile
├── Makefile
├── DEMO.md
├── Dockerfile
├── docker-compose.yml
└── README.md
```

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `DB_PATH` | `cortex.db` | SQLite database file path |
| `OLLAMA_URL` | `http://localhost:11434` | Ollama server URL |
| `OLLAMA_MODEL` | `llama3.1:8b` | Model to use for tool-calling and agent assist |
| `WHISPER_URL` | `http://localhost:8001` | Whisper STT microservice URL |
| `SENTIMENT_URL` | `http://localhost:5001` | DistilBERT sentiment microservice URL |

---

## Roadmap

- [x] Project scaffold, Go module, core dependencies
- [x] Domain models — Call, Agent, Transcript, Event
- [x] SQLite store with schema migrations
- [x] HTTP REST API (`/api/calls`, `/api/agents`, `/api/queues`, `/api/transcripts`)
- [x] Docker + Docker Compose setup
- [x] Call simulation engine — goroutine-based state machine
- [x] Random call generator with realistic caller data and queue routing
- [x] Fake transcript generator — queue-specific dialogue during active calls
- [x] WebSocket hub — real-time event broadcast to all connected clients
- [x] MCP server with 7 tools (stdio transport — Claude Desktop / Cursor compatible)
- [x] Ollama HTTP client with tool-call support
- [x] Agentic tool-calling loop — multi-turn, up to 5 rounds
- [x] `/api/chat` endpoint — supervisor natural language queries
- [x] Supervisor dashboard — live queues, agent tiles, AI chat panel
- [x] Proactive anomaly monitor — SLA, sentiment, queue overload, agent shortage
- [x] AI advisory generation — LLM writes 2–3 sentence supervisor brief on anomaly
- [x] Whisper transcription pipeline — FastAPI microservice, on-prem STT via `/api/transcribe`
- [x] HuggingFace sentiment microservice — DistilBERT, Docker service, real NLP scoring + EMA
- [x] Agent assist — real-time Llama 3.1:8b response suggestions broadcast per customer line
- [x] Unit tests — engine + MCP tools, SQLite in-memory, race-safe
- [x] Makefile — run, build, test, docker targets
- [x] DEMO.md — step-by-step hiring-manager demo script

---

## License

MIT — built by [Abhiram Hatwar](https://github.com/abhiramhatwar)
