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

No cloud. No external API calls. No data leaves your server.

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

| Feature | Description |
|---|---|
| **MCP Server** | 7 tools exposable to any local LLM via the Model Context Protocol |
| **Natural Language Queries** | Supervisor chats in plain English — LLM calls the right tools |
| **Live Call Queue** | Real-time WebSocket feed of active calls, wait times, SLA breaches |
| **Agent Monitoring** | Status, call count, avg handle time, sentiment history per agent |
| **Real-Time Transcription** | Whisper-powered, streamed line by line during active calls |
| **Sentiment Analysis** | Per-call customer mood, tracked over time, alerts on drops |
| **Proactive Alerts** | LLM autonomously monitors the queue and fires alerts without being asked |
| **Post-Call Summaries** | Structured JSON summary (issue, resolution, follow-up) auto-generated |
| **Agent Assist** | Real-time suggested responses surfaced to the agent during a live call |
| **Fully On-Prem** | Ollama + Whisper + HuggingFace — everything runs on your own hardware |

---

## How It Works

### 1. MCP Tools

The MCP server exposes the contact center as a set of callable tools:

| Tool | What It Does |
|---|---|
| `get_queue_status` | Returns active queue depth, wait times, SLA breach count |
| `get_agent_states` | Returns all agents with status, current call, handle time |
| `get_call_transcript` | Returns live or historical transcript for a call ID |
| `get_sentiment_report` | Returns sentiment scores across all active calls |
| `route_call` | Moves a call to a specific agent or queue |
| `flag_call` | Marks a call for QA review with a reason |
| `summarize_call` | Triggers post-call AI summary generation |

### 2. The LLM Loop

When a supervisor asks a question, here's what happens under the hood:

```
1. Supervisor types: "Who needs help right now?"
2. Message sent to Ollama (Llama 3.1:8b — local)
3. Ollama decides which MCP tools to call
4. cortex-cc executes: get_agent_states + get_sentiment_report
5. Results returned to Ollama
6. Ollama reasons over the data and responds in plain English
7. Answer shown to supervisor in under 2 seconds
```

### 3. Proactive Monitoring

cortex-cc runs a background loop that feeds queue state to the LLM every 60 seconds. If the LLM detects an anomaly (SLA breach spike, sentiment crash, queue buildup), it fires an alert to the supervisor dashboard without being asked.

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Browser (Supervisor)                  │
│         Chat Interface  │  Live Dashboard                │
└──────────┬──────────────┴──────────────┬────────────────┘
           │ REST / WebSocket             │ WebSocket events
           ▼                             ▼
┌─────────────────────────────────────────────────────────┐
│                    Go API Server                         │
│                                                          │
│  ┌─────────────┐   ┌──────────────┐  ┌──────────────┐  │
│  │ MCP Server  │   │  WebSocket   │  │  REST API    │  │
│  │             │   │     Hub      │  │  /api/calls  │  │
│  │ 7 tools     │   │  broadcast   │  │  /api/agents │  │
│  └──────┬──────┘   └──────────────┘  └──────────────┘  │
│         │                                                │
│  ┌──────▼──────┐   ┌──────────────┐  ┌──────────────┐  │
│  │   Ollama    │   │ Call Engine  │  │   SQLite     │  │
│  │  Client     │   │ (goroutines) │  │   Store      │  │
│  │ Llama 3.1   │   │ fake SIP +   │  │              │  │
│  │   (local)   │   │ queue sim    │  │              │  │
│  └─────────────┘   └──────────────┘  └──────────────┘  │
│                                                          │
│  ┌─────────────┐   ┌──────────────┐                     │
│  │   Whisper   │   │  HuggingFace │                     │
│  │ Transcriber │   │  Sentiment   │                     │
│  │  (on-prem)  │   │  (on-prem)   │                     │
│  └─────────────┘   └──────────────┘                     │
└─────────────────────────────────────────────────────────┘
```

---

## Tech Stack

| Layer | Technology | Why |
|---|---|---|
| Backend | **Go 1.25** | High concurrency for real-time call events |
| MCP | **mark3labs/mcp-go** | MCP server implementation in Go |
| LLM | **Ollama + Llama 3.1:8b** | Free, local, tool-calling capable |
| Transcription | **OpenAI Whisper** | Best open-source STT, runs fully on-prem |
| Sentiment | **HuggingFace Transformers** | Free, local sentiment pipeline |
| Database | **SQLite** | Zero-dependency, embedded, perfect for single-node |
| Real-time | **WebSocket (gorilla/websocket)** | Live push to supervisor dashboard |
| Frontend | **HTML + TailwindCSS** | Lightweight, no build step needed |
| Containers | **Docker + Docker Compose** | One-command deploy |

---

## Quick Start

### Prerequisites

- [Docker + Docker Compose](https://docs.docker.com/get-docker/)
- [Ollama](https://ollama.com) installed locally

### 1. Pull the LLM model

```bash
ollama pull llama3.1:8b
```

### 2. Clone and start

```bash
git clone https://github.com/abhiramhatwar/cortex-cc.git
cd cortex-cc
docker-compose up
```

### 3. Open the supervisor dashboard

```
http://localhost:8080
```

That's it. The call simulation engine starts automatically, populating the dashboard with live agents, queues, and calls.

---

## Usage Examples

### Natural Language Queries

```
"Which calls are at risk of breaching SLA?"
→ cortex calls get_queue_status
→ "3 calls have been waiting over 4 minutes in the Billing queue."

"Route call C-1042 to Sarah."
→ cortex calls route_call(call_id="C-1042", agent_id="sarah")
→ "Done. Sarah has been notified and the call is being transferred."

"What are customers complaining about today?"
→ cortex calls get_call_transcript for recent calls + get_sentiment_report
→ "Most complaints today are around invoice delays and account access issues."

"Flag the last call with negative sentiment for QA."
→ cortex calls get_sentiment_report + flag_call
→ "Call C-1039 has been flagged for QA review."
```

### REST API

```bash
# Get all active calls
curl http://localhost:8080/api/calls

# Get all agents and their status
curl http://localhost:8080/api/agents

# Get live transcript for a call
curl http://localhost:8080/api/calls/C-1042/transcript

# Get post-call summary
curl http://localhost:8080/api/calls/C-1042/summary

# Get queue stats
curl http://localhost:8080/api/queues
```

---

## Why This Matters

| | Mitel CX 2.0 | cortex-cc |
|---|---|---|
| AI Engine | Talkative AI (cloud) | Ollama + Llama 3.1 (local) |
| Data leaves server | Yes | Never |
| HIPAA / PCI compliant | No | Yes |
| MCP support | None | Full |
| Swap LLM model | No | Yes (any Ollama model) |
| Monthly cost | $65–85/agent | $0 |
| Open source | No | Yes |

---

## Project Structure

```
cortex-cc/
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── internal/
│   ├── config/                  # Env-based configuration
│   ├── models/                  # Domain types (Call, Agent, Transcript...)
│   ├── store/                   # SQLite persistence layer
│   ├── server/                  # HTTP REST handlers
│   ├── engine/                  # Call simulation engine (goroutines)
│   ├── websocket/               # Real-time WebSocket hub
│   ├── mcp/                     # MCP server + 7 tools
│   ├── llm/                     # Ollama client + tool-calling loop
│   ├── transcription/           # Whisper integration
│   └── sentiment/               # HuggingFace sentiment client
├── web/                         # Supervisor dashboard (HTML + JS)
├── sentiment/                   # Python sentiment microservice
├── Dockerfile
├── docker-compose.yml
└── README.md
```

---

## Roadmap

- [x] Project scaffold, config, domain models
- [x] SQLite store with migrations
- [x] HTTP REST API
- [x] Docker + Docker Compose setup
- [ ] Call simulation engine (goroutines, state machines)
- [ ] WebSocket real-time hub
- [ ] MCP server with 7 tools
- [ ] Ollama tool-calling integration
- [ ] Whisper transcription pipeline
- [ ] HuggingFace sentiment analysis
- [ ] Supervisor dashboard UI
- [ ] Proactive anomaly detection
- [ ] Post-call auto-summaries
- [ ] Agent assist (real-time suggestions)

---

## License

MIT — built by [Abhiram Hatwar](https://github.com/abhiramhatwar)
