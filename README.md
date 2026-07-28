# cortex-cc

**On-Prem AI Copilot for Contact Centers — powered by MCP + Ollama + Llama 3.1**

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![MCP](https://img.shields.io/badge/MCP-Model%20Context%20Protocol-7C3AED)](https://modelcontextprotocol.io)
[![Ollama](https://img.shields.io/badge/LLM-Ollama%20%2B%20Llama%203.1:8b-F97316)](https://ollama.com)
[![Whisper](https://img.shields.io/badge/STT-OpenAI%20Whisper-10B981)](https://github.com/openai/whisper)
[![HuggingFace](https://img.shields.io/badge/NLP-DistilBERT-FFD21E)](https://huggingface.co)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker)](https://docker.com)
[![License](https://img.shields.io/badge/license-MIT-22C55E)](LICENSE)
[![Commits](https://img.shields.io/badge/commits-30-6366F1)](https://github.com/abhiramhatwar/cortex-cc)

---

## What Is This?

cortex-cc is a **fully on-prem AI copilot** that sits on top of any contact center platform and gives supervisors the ability to ask questions, take actions, and receive real-time intelligence — entirely using free, self-hosted AI models.

No cloud. No API keys. No data ever leaves your server.

```
Supervisor types:  "Which agents are struggling and what should I do right now?"

cortex-cc:
  → calls get_agent_states()      (live agent data from engine)
  → calls get_sentiment_report()  (NLP scores from DistilBERT)
  → Llama 3.1:8b reasons over results
  → "Agent Priya has handled 12 calls today and her last 4 calls
     all scored below -0.4 sentiment. Consider moving her to a
     break rotation and redistributing the Billing queue to James."

Zero bytes left the building.
```

---

## The Problem

### Why Contact Centers Can't Use Cloud AI

Mitel CX 2.0, the industry's dominant contact center platform, ships AI features through **Talkative AI** — a third-party, cloud-based integration.

This creates a hard blocker for entire industries:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    WHO CANNOT USE CLOUD AI                          │
├──────────────────┬──────────────────┬───────────────────────────────┤
│   Healthcare     │   Banking/Fin    │   Government / Defence        │
│                  │                  │                               │
│  Patient names   │  Account nums    │  Classified caller info       │
│  Call reasons    │  Credit details  │  Citizen PII                  │
│  Medications     │  Transaction IDs │  Case numbers                 │
│                  │                  │                               │
│  → HIPAA         │  → PCI-DSS       │  → FedRAMP / IL4+             │
│  → HITECH        │  → SOX           │  → ITAR                       │
└──────────────────┴──────────────────┴───────────────────────────────┘
```

These three verticals represent **60%+ of enterprise contact center seats**.

Every single one of them receives a "no" when they ask Mitel for on-prem AI. That's the gap cortex-cc fills.

### The Market Gap in One Table

| Capability | Mitel CX 2.0 | RingCentral RingSense | **cortex-cc** |
|---|---|---|---|
| AI engine | Talkative AI (cloud) | Proprietary cloud | Ollama + Llama 3.1 (local) |
| Data leaves server | Yes | Yes | **Never** |
| HIPAA / PCI compliant | No | No | **Yes** |
| MCP support | None | None | **Full — 7 tools, stdio binary** |
| Swap the LLM | No | No | **Yes — any Ollama model** |
| Real-time sentiment | No | Yes (cloud) | **Yes (on-prem DistilBERT)** |
| Agent assist suggestions | No | Yes (cloud) | **Yes (on-prem Llama 3.1)** |
| Proactive anomaly alerts | Basic | Yes | **Yes + AI advisory** |
| Post-call QA scoring | Manual | Cloud AI | **Yes (on-prem Llama 3.1)** |
| Speech-to-text | Vendor only | Vendor only | **Yes (on-prem Whisper)** |
| Monthly cost per agent | $65–$85 | $35–$75 | **$0** |
| Open source | No | No | **Yes (MIT)** |

---

## The Solution

cortex-cc is an **MCP server** built in Go. It wraps a contact center's live data as callable tools, runs a local LLM to reason over those tools, and delivers real-time intelligence to supervisors — with zero cloud dependency.

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         YOUR SERVER ROOM                                 │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    cortex-cc (Go binary)                        │    │
│  │                                                                  │    │
│  │   Supervisor asks a question in plain English                   │    │
│  │          ↓                                                       │    │
│  │   [Llama 3.1:8b] decides which tools to call                   │    │
│  │          ↓                                                       │    │
│  │   [MCP Tool Registry] executes: get_queue_status,              │    │
│  │                                  get_sentiment_report           │    │
│  │          ↓                                                       │    │
│  │   [Llama 3.1:8b] reasons over live data                        │    │
│  │          ↓                                                       │    │
│  │   Supervisor gets a plain-English answer with action items      │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  All AI: Ollama (local)  ·  All data: SQLite (local)                    │
│  All NLP: DistilBERT (local)  ·  All STT: Whisper (local)               │
│                                                                          │
│                     ←  Internet boundary  →  NOTHING crosses this       │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## System Architecture

```
┌──────────────────── Browser: Supervisor Dashboard ─────────────────────┐
│                                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────┐  ┌───────────────┐  │
│  │  Stats Row   │  │  Call Queue  │  │  Agents  │  │  AI Chat Box  │  │
│  │  (live KPIs) │  │  (real-time) │  │  Panel   │  │  (copilot)    │  │
│  └──────┬───────┘  └──────┬───────┘  └────┬─────┘  └───────┬───────┘  │
│         │                 │               │                 │           │
│  ┌──────┴─────────────────┴───────────────┴─────────────────┴───────┐  │
│  │              WebSocket  ws://localhost:8080/ws                    │  │
│  └──────────────────────────┬────────────────────────────────────────┘  │
│                             │ POST /api/chat, GET /api/calls, etc.       │
└─────────────────────────────┼──────────────────────────────────────────-┘
                              │  HTTP
┌─────────────────────────────▼──────────────────────────────────────────┐
│                      Go HTTP Server  :8080                             │
│                                                                         │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │                         Router (net/http)                         │ │
│  │  GET /health  GET /ws  POST /api/chat  POST /api/chat/reset       │ │
│  │  GET /api/calls  GET /api/agents  GET /api/queues                 │ │
│  │  GET /api/calls/{id}/transcript  GET /api/calls/{id}/summary      │ │
│  │  GET /api/calls/{id}/score  POST /api/calls/{id}/flag             │ │
│  │  POST /api/calls/{id}/route  POST /api/transcribe                 │ │
│  │  GET /  (static → web/index.html)                                 │ │
│  └────────────┬─────────────────────────────────────┬───────────────┘ │
│               │                                     │                  │
│  ┌────────────▼──────────┐           ┌──────────────▼──────────────┐  │
│  │     LLM Loop          │           │      WebSocket Hub           │  │
│  │  (agentic, max 5 rnds)│           │  (fan-out broadcast)         │  │
│  │  multi-turn history   │           │  gorilla/websocket           │  │
│  └────────────┬──────────┘           └──────────────▲──────────────┘  │
│               │                                     │                  │
│  ┌────────────▼──────────┐           ┌──────────────┴──────────────┐  │
│  │   MCP Tool Registry   │           │       Call Engine            │  │
│  │  7 tools, shared by   │           │  goroutine state machine     │  │
│  │  Ollama + MCP server  │           │  tick=2s, gen=8s, tx=6s     │  │
│  └────────────┬──────────┘           └──────────────┬──────────────┘  │
│               │                                     │                  │
│  ┌────────────▼──────────┐           ┌──────────────▼──────────────┐  │
│  │   Ollama HTTP Client  │           │    Proactive Monitor         │  │
│  │  POST /api/chat       │           │    60s poll, auto-alert      │  │
│  │  90s timeout          │           │    + LLM advisory            │  │
│  └────────────┬──────────┘           └──────────────┬──────────────┘  │
│               │                                     │                  │
│  ┌────────────▼──────────┐           ┌──────────────▼──────────────┐  │
│  │    SQLite (no CGO)    │           │     Agent Assist Service     │  │
│  │  calls, agents,       │           │  40s cooldown, last-6 lines  │  │
│  │  transcripts,         │           │  → Llama suggestion          │  │
│  │  summaries,           │           └──────────────┬──────────────┘  │
│  │  call_scores          │                          │                  │
│  └───────────────────────┘           ┌──────────────▼──────────────┐  │
│                                      │     QA Scorer (internal/qa)  │  │
│                                      │  OnCallCompleted hook        │  │
│                                      │  transcript → Llama → score  │  │
│                                      │  1-10: empathy, resolution,  │  │
│                                      │  professionalism, overall    │  │
│                                      └─────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────┘
         │                          │
         ▼                          ▼
┌─────────────────┐      ┌──────────────────────┐
│  Ollama :11434  │      │  HuggingFace Sent.   │
│  Llama 3.1:8b   │      │  Service :5001        │
│  (local Docker) │      │  DistilBERT on CPU    │
└─────────────────┘      └──────────────────────┘
                                    │
                         ┌──────────────────────┐
                         │  Whisper STT :8001    │
                         │  OpenAI Whisper base  │
                         │  (local Docker)       │
                         └──────────────────────┘

Standalone MCP stdio binary (out-of-process):

┌─────────────────────────────────────────────────────────────┐
│  bin/cortex-mcp  (cmd/mcp-server)                           │
│                                                             │
│  MCP Client (Cursor / VS Code Copilot)                      │
│       │  stdio                                              │
│       ▼                                                     │
│  MCPServer (mark3labs/mcp-go)                               │
│       │  Execute(tool, args)                                │
│       ▼                                                     │
│  HTTPExecutor  ──────► GET/POST http://localhost:8080/api/  │
│                         (calls the running cortex-cc server) │
└─────────────────────────────────────────────────────────────┘
```

---

## How Each Subsystem Works

### 1. Call Engine

The engine is a goroutine-based state machine that simulates a live contact center. In a production deployment this would connect to the PBX via SIP/TAPI; in this build it generates realistic traffic for demonstration.

**Three concurrent goroutines run on fixed tickers:**

```
  callGenInterval = 8s          tickInterval = 2s         transcriptInterval = 6s
         │                            │                            │
         ▼                            ▼                            ▼
  generateCall()              tick()                  generateTranscriptLines()
         │                            │                            │
  Picks a random:             For every call:          For every active call:
  - queue (Sales/             - Queued → advance        - Pick speaker
    Billing/Support)            wait timer               (50/50 agent/customer)
  - caller name               - SLA breach at 120s     - Pick from queue-specific
  - caller number             - 3% chance to abandon     dialogue bank
  - sentiment start:          - Auto-assign agent       - Store in SQLite
    0.2 to 0.8                  if available            - If customer line:
                              - Active → advance          → UpdateSentiment()
  Emits: call_queued            talk timer                → OnCustomerLine hook
                              - 4% chance to complete
                              Emits: call_answered,
                                     call_updated,
                                     call_completed,
                                     sla_breached
```

**Call State Machine:**

```
                    ┌──────────────────────────────────────┐
                    │                                      │
   arrival          │    available agent found             │
  ─────────►  QUEUED ─────────────────────────►  ACTIVE ──┘
                    │                                      │
                    │  wait > 30s, 3% per tick             │  talk > 60s, 4% per tick
                    ▼                                      ▼
               ABANDONED                             COMPLETED
```

**SLA logic:** Any call waiting over 120 seconds sets `SLABreached = true` and emits `sla_breached`. The monitor picks this up within its next 60-second poll.

**Sentiment EMA** (exponential moving average, α = 0.3):
```
new_sentiment = 0.7 × prev_sentiment + 0.3 × nlp_score
```
This means a single very negative line won't crater the score — the signal smooths over the conversation arc.

---

### 2. WebSocket Hub

Every event the engine emits is broadcast to all connected dashboard clients in real time.

```
  Engine goroutine
       │
       │  emit("call_queued", call)
       ▼
  Events chan *models.Event   ← buffered (cap 256)
       │
       │  hub.Run() drains this channel
       ▼
  Hub.Broadcast(evt)
       │
       ├──► client A  send chan (cap 64) → writePump → WS frame
       ├──► client B  send chan (cap 64) → writePump → WS frame
       └──► client C  send chan (cap 64) → writePump → WS frame

  Slow client? Message is dropped (select/default). No backpressure.
```

The dashboard's JavaScript `handleEvent()` switches on `evt.type` and updates the DOM. No polling. Everything is push.

---

### 3. MCP Tool Registry

The tool registry is the bridge between the LLM and live contact center data. It serves two consumers simultaneously:

```
  ┌─────────────────────────────────────────────────────┐
  │                 ToolRegistry                         │
  │                                                      │
  │  Definitions() ──► []ToolDef (JSON schemas)          │
  │  Execute(name, args) ──► (string JSON, error)        │
  │                                                      │
  │      Consumer A                Consumer B            │
  │  ┌───────────────┐        ┌──────────────────┐       │
  │  │  Ollama Loop  │        │   MCP stdio srv  │       │
  │  │  (chat API)   │        │  (Claude Desktop │       │
  │  │               │        │   Cursor, etc.)  │       │
  │  └───────────────┘        └──────────────────┘       │
  └─────────────────────────────────────────────────────┘
```

Both consumers call the exact same `Execute()` method. The tool registry doesn't know or care who is calling it — it just reads from the engine and store, builds a JSON string, and returns it.

---

### 4. Agentic Loop (Multi-Turn Tool Calling)

```
  User: "Flag the most distressed caller for QA."

  Round 1:
    → Ollama sees: user message + 7 tool definitions
    → Ollama responds with: tool_call{ get_sentiment_report() }
    → cortex-cc executes tool, appends result to history

  Round 2:
    → Ollama sees: history + sentiment data (C-a1b2c3 → -0.82)
    → Ollama responds with: tool_call{ flag_call(C-a1b2c3, "distressed caller") }
    → cortex-cc executes tool, appends result to history

  Round 3:
    → Ollama sees: flag confirmation
    → Ollama responds with: text (no tool calls)
    → "Call C-a1b2c3 (Michael Johnson, Billing) has been flagged
       for QA review. Sentiment score: -0.82."

  DONE — 2 tool rounds, 1 text reply.
  Maximum allowed: 5 rounds (prevents infinite loops).
```

**Conversation history** persists across turns. A supervisor can ask:
```
Turn 1: "Who is the busiest agent?"        → Ollama calls get_agent_states
Turn 2: "Give them a break."               → Ollama already knows who "them" is
Turn 3: "Now route their calls to Sarah."  → Full context retained
```

`POST /api/chat/reset` clears history back to the system prompt only.

---

### 5. Sentiment Analysis Pipeline

```
  Call goes active
       │
       │  transcript ticker fires (every 6s)
       ▼
  customer line selected from queue dialogue bank
       │
       ├──────────────────────────────────────────────┐
       │  goroutine 1: UpdateSentiment()              │
       │                                              ▼
       │         POST http://sentiment:5001/analyze
       │         { "text": "I've called 3 times already!" }
       │                     │
       │                     ▼
       │         DistilBERT (distilbert-base-uncased-finetuned-sst-2-english)
       │         → { "score": -0.91, "label": "negative", "confidence": 0.91 }
       │                     │
       │                     ▼
       │         EMA update: c.Sentiment = 0.7 × prev + 0.3 × (-0.91)
       │         Broadcasts: sentiment_update event via WebSocket
       │
       └──────────────────────────────────────────────┐
          goroutine 2: OnCustomerLine hook            │
                       → assist.ProcessLine()         ▼
                         (see Agent Assist below)
```

**Scoring scale:**

```
  -1.0 ────────────────────────────────────── +1.0
   │                    │                       │
  Very               Neutral                  Very
  Negative            (0.0)                 Positive

  < -0.3 → dashboard bar turns red
  < -0.4 → monitor fires warning alert
  < -0.6 → monitor fires critical alert
```

**Graceful degradation:** If the sentiment service is not running at startup, `sc` is set to `nil`. The engine falls back to a random float drift on active calls (`randFloat(-0.05, 0.05)` per tick). The dashboard still shows a bar — it just drifts rather than reflecting real NLP.

---

### 6. Whisper Transcription Pipeline

```
  Supervisor uploads audio file via API
       │
       ▼
  POST /api/transcribe
  Content-Type: multipart/form-data
  Fields: audio (file), call_id (string), speaker (string)
       │
       ▼
  server.go: reads file bytes (max 32 MB)
       │
       ▼
  transcriber.TranscribeBytes(audio, filename)
       │
       ▼
  POST http://whisper:8001/transcribe
  (multipart audio upload)
       │
       ▼
  Whisper service (Python + openai-whisper):
    1. Write bytes to temp file
    2. model.transcribe(path)       ← Whisper base model on CPU
    3. Return { text, segments[], language, duration, elapsed }
    4. Delete temp file
       │
       ▼
  For each segment with timestamps:
    store.InsertTranscript(&Transcript{
        CallID:    "C-a1b2c3",
        Speaker:   "customer",
        Text:      segment.Text,
        Timestamp: baseTime + segment.Start,
    })
       │
       ▼
  Response: { text, segments, language, duration, elapsed, stored }
```

**Supported formats:** `.wav` `.mp3` `.m4a` `.ogg` `.flac` `.webm` `.mp4`

**Models available** (set via `WHISPER_MODEL` env var):

| Model | Size | Speed (CPU) | Accuracy |
|---|---|---|---|
| `tiny` | 39 MB | ~10x realtime | Basic |
| `base` | 74 MB | ~7x realtime | Good (default) |
| `small` | 244 MB | ~4x realtime | Better |
| `medium` | 769 MB | ~2x realtime | Very good |
| `large` | 1.5 GB | ~1x realtime | Best |

---

### 7. Agent Assist Service

Every time a customer speaks on a live call, the assist service generates a suggested response for the agent — rate-limited so it doesn't flood.

```
  customer line detected by transcript generator
       │
       ▼
  eng.OnCustomerLine(call, "my internet has been down for 3 days")
       │  (function field — no import cycle)
       ▼
  assist.ProcessLine(call, text)
       │
       ├── rate limiter check
       │   lastSug[call.ID] = 42 seconds ago → cooldown = 40s → BLOCKED
       │   lastSug[call.ID] = 55 seconds ago → cooldown elapsed → PROCEED
       │
       ▼
  store.GetTranscriptByCallID(call.ID)   ← last 6 lines for context
       │
       ▼
  Build prompt:
    Queue: Support
    Recent conversation:
    [AGENT]: Thank you for calling Support, what issue are you experiencing?
    [CUSTOMER]: Hi, my internet has been down for 3 days.
    [AGENT]: Let me run a diagnostic on your connection.
    [CUSTOMER]: I've already restarted it three times, it's not helping.
    Customer just said: "my internet has been down for 3 days"
    Suggested agent response:
       │
       ▼
  POST http://ollama:11434/api/chat
  System: "Max 2 sentences. Professional and empathetic.
           No hollow fillers. Return ONLY the suggestion."
       │
       ▼
  Suggestion: "I sincerely apologize for the prolonged disruption —
               I can see this is impacting you significantly, so let me
               escalate this to our infrastructure team right now."
       │
       ▼
  hub.Broadcast({ type: "agent_assist", payload: {
      call_id, agent_id, queue, trigger, suggestion
  }})
       │
       ▼
  Dashboard: suggestion card slides in, auto-dismisses in 30s
```

**Why 40-second cooldown?** A 6-second transcript tick means multiple customer lines arrive quickly. Without rate-limiting, every line would trigger an LLM call — flooding Ollama and producing incoherent rapid-fire suggestions. 40 seconds gives one useful suggestion per customer turn.

---

### 8. Post-Call QA Scoring

Every time a call completes, cortex-cc automatically scores the agent's performance using a local Llama 3.1 inference — no human reviewer needed, no cloud.

```
  call transitions to "completed"
       │
       ▼
  engine.completeCall() fires
       │
       ├── emits "call_completed" WebSocket event
       │
       └── eng.OnCallCompleted(copy of call)   ← hook (no import cycle)
                │  goroutine — doesn't block the engine tick
                ▼
         qa.Scorer.scoreAsync(call)
                │
                ▼
         store.GetTranscriptByCallID(call.ID)
                │  if no transcript → skip quietly
                ▼
         Build prompt:
           Call ID: C-a1b2c3 | Queue: Billing | Talk: 147s | Sentiment: -0.41

           Transcript:
           [AGENT]: Thank you for calling Billing, how can I help?
           [CUSTOMER]: I was charged twice and I'm furious.
           [AGENT]: I sincerely apologize — I can see the duplicate charge...
                │
                ▼
         llm.Client.OneShot(system_prompt, transcript_prompt)
           System: "Score empathy, resolution, professionalism, overall (1-10).
                    Return ONLY valid JSON, no markdown."
                │
                ▼
         Llama 3.1:8b responds:
           {
             "empathy": 9,
             "resolution": 8,
             "professionalism": 9,
             "overall": 9,
             "notes": "Agent handled an angry customer with genuine empathy
                       and resolved the billing issue efficiently."
           }
                │
                ▼
         parseScore() — extracts JSON, clamps each field 1-10
                │
                ▼
         store.InsertCallScore(score)  → call_scores table
                │
                ▼
         hub.Broadcast({ type: "call_scored", payload: score })
                │
                ▼
         Dashboard: QA card slides in with color-coded scores
         API:       GET /api/calls/{id}/score returns the score
```

**Score color coding:**

```
  1 ────────── 4 ────────── 7 ────────── 10
  │            │            │            │
  Red          │   Amber    │   Green    │
  Poor         Acceptable   Excellent
```

**Why a goroutine + hook pattern?** The QA scorer is slow (LLM inference takes 5-30 seconds). Calling it synchronously inside the engine tick would freeze all call processing. The `OnCallCompleted` hook fires the scorer in a background goroutine with a copy of the completed call. The engine continues its tick loop with zero latency.

**Graceful degradation:** If Ollama is offline, `llm.OneShot()` returns an error — the scorer logs it and skips silently. No crash, no panic. Calls still complete normally.

---

### 9. Standalone MCP Stdio Server

`bin/cortex-mcp` is a separate binary that exposes all 7 contact center tools over the Model Context Protocol stdio transport. Unlike the in-process MCP server (which shares the engine's memory), this binary runs as an independent process and communicates with a running cortex-cc instance via HTTP.

```
  MCP Client (Cursor IDE, VS Code Copilot, etc.)
       │
       │  spawns  bin/cortex-mcp  (stdio)
       ▼
  cortex-mcp process
       │
       │  tool_call{ "get_queue_status" }
       ▼
  HTTPExecutor.Execute("get_queue_status")
       │
       ▼
  GET http://localhost:8080/api/calls
       │
       ▼
  running cortex-cc server
  (contacts its in-memory engine)
       │
       ▼
  JSON queue stats returned to MCP client
```

**Why a standalone binary?** MCP clients spawn the server as a child process and communicate over stdin/stdout. A standalone binary lets any MCP client (Cursor, VS Code, Claude Desktop, any MCP-compatible tool) query live contact center data without modifying the cortex-cc server.

**Configuration:**

```bash
# Build the binary
make build-mcp

# Run (connect to non-default server)
CORTEX_URL=http://prod-server:8080 ./bin/cortex-mcp

# Cursor integration (~/.cursor/mcp.json):
{
  "mcpServers": {
    "cortex-cc": {
      "command": "/path/to/bin/cortex-mcp",
      "env": { "CORTEX_URL": "http://localhost:8080" }
    }
  }
}
```

**Write operations supported:** `flag_call` and `route_call` are exposed as MCP write tools — an MCP client can say "Flag C-abc123 as escalation" and cortex-mcp will POST to the cortex-cc API to execute it.

---

### 10. Proactive Monitor

The monitor runs independently of any user query. Every 60 seconds it calls the tool registry directly, computes anomalies, and fires alerts automatically.

```
  60s ticker fires
       │
       ▼
  detectAnomalies()
       │
       ├── Execute("get_queue_status")
       │     For each queue:
       │       SLABreaches ≥ 1 → warning alert
       │       SLABreaches ≥ 3 → critical alert
       │       WaitingCalls ≥ 5 → queue overload warning
       │
       ├── Execute("get_sentiment_report")
       │     NegativeCalls ≥ 3 OR AvgSentiment < -0.4 → warning
       │     AvgSentiment < -0.6 → critical
       │
       └── Execute("get_agent_states")
             AvailableCount / Total ≤ 10% → critical agent shortage

  For each anomaly:
    hub.Broadcast({ type: "alert", payload: { level, title, detail } })

  If ANY anomalies:
    go generateAdvisory(anomalies)
         │
         ▼
    loop.Chat("Proactive monitor detected:\n" +
              "- [critical] SLA breach in Billing: 4 calls...\n" +
              "Provide a 2-3 sentence supervisor advisory...")
         │
         ▼
    hub.Broadcast({ type: "alert", source: "ai_advisory", detail: reply })
```

**Alert levels in the dashboard:**

```
  Red banner  → operational alert (SLA, queue, shortage)
  Violet banner → AI advisory (LLM-generated action recommendation)
```

---

## Feature Overview

| Feature | What It Does | Key Detail |
|---|---|---|
| **MCP Stdio Server** | Standalone binary exposing 7 tools over stdio | Any MCP client: Cursor, VS Code, etc. |
| **MCP Tool Registry** | In-process tool calling by Ollama loop | Shared `Execute()` — no duplication |
| **AI Copilot Chat** | Natural language Q&A over live data | Agentic, up to 5 tool rounds |
| **Multi-Turn History** | Conversation context persists across questions | Reset via `POST /api/chat/reset` |
| **Post-Call QA Scoring** | Scores every completed call 1-10 on 4 dimensions | Async, on-prem Llama 3.1, WebSocket push |
| **Live Call Queue** | Real-time table of all active/queued calls | WebSocket push, no polling |
| **Agent Monitoring** | Status, calls handled, avg handle time per agent | Polled every 5s via REST |
| **SLA Tracking** | Breach detection at 120s wait threshold | Immediate WS event + visual flag |
| **Sentiment Scoring** | Per-call NLP score updated on every customer line | DistilBERT EMA, [-1.0, 1.0] |
| **Proactive Monitor** | Autonomous anomaly detection every 60s | No supervisor query needed |
| **AI Advisories** | LLM-written action recommendations on anomaly | 2-3 sentence, concrete |
| **Agent Assist** | Real-time response suggestions during live calls | 40s cooldown per call |
| **Whisper STT** | Upload audio → timestamped transcript segments | Stored in SQLite by call |
| **Call Flagging** | Mark calls for QA with a reason | Via chat, MCP, or direct API |
| **Call Routing** | Transfer a call to a specific agent | Via chat, MCP, or direct API |
| **Post-Call Summary** | Structured JSON: issue, resolution, follow-up | LLM-generated |
| **Supervisor Dashboard** | Full dark-themed HTML UI, no build step | Tailwind CSS, WebSocket |
| **Docker Compose** | One-command deploy of all 4 services | Ollama, Whisper, Sentiment, cortex |
| **On-Prem Only** | Zero external API calls — everything local | No API keys needed anywhere |

---

## Database Schema

cortex-cc uses **SQLite** via `modernc.org/sqlite` (pure Go, no CGO required). The database is a single file (`cortex.db` by default). All tables are created via a single `migrate()` call at startup — no migration framework needed.

### Tables

```
┌─────────────────────────────────────────────────────────────────┐
│  TABLE: calls                                                   │
├────────────────┬───────────────┬────────────────────────────────┤
│  Column        │  Type         │  Notes                         │
├────────────────┼───────────────┼────────────────────────────────┤
│  id            │  TEXT PK      │  "C-a1b2c3" format             │
│  caller_number │  TEXT         │  e.g. "416-234-5678"           │
│  caller_name   │  TEXT         │  nullable                      │
│  agent_id      │  TEXT         │  FK → agents.id, nullable      │
│  queue_name    │  TEXT         │  Sales | Billing | Support     │
│  status        │  TEXT         │  queued|active|completed|etc.  │
│  wait_seconds  │  INTEGER      │  time in queue                 │
│  talk_seconds  │  INTEGER      │  time on active call           │
│  sentiment     │  REAL         │  -1.0 to 1.0, EMA updated      │
│  sla_breached  │  INTEGER      │  0 or 1 (bool)                 │
│  flagged       │  INTEGER      │  0 or 1 (bool)                 │
│  flag_reason   │  TEXT         │  nullable                      │
│  started_at    │  DATETIME     │  call arrival time             │
│  ended_at      │  DATETIME     │  nullable — set on completion  │
└────────────────┴───────────────┴────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  TABLE: agents                                                  │
├────────────────┬───────────────┬────────────────────────────────┤
│  Column        │  Type         │  Notes                         │
├────────────────┼───────────────┼────────────────────────────────┤
│  id            │  TEXT PK      │  UUID                          │
│  name          │  TEXT         │  e.g. "Sarah Mitchell"         │
│  status        │  TEXT         │  available|busy|on_break|etc.  │
│  current_call_id│ TEXT         │  nullable                      │
│  calls_handled │  INTEGER      │  today's count                 │
│  avg_handle_time│ REAL         │  running average in seconds    │
│  skills        │  TEXT         │  JSON array: ["Sales","Billing"]│
└────────────────┴───────────────┴────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  TABLE: transcripts                                             │
├────────────────┬───────────────┬────────────────────────────────┤
│  Column        │  Type         │  Notes                         │
├────────────────┼───────────────┼────────────────────────────────┤
│  id            │  TEXT PK      │  UUID                          │
│  call_id       │  TEXT         │  FK → calls.id                 │
│  speaker       │  TEXT         │  "agent" or "customer"         │
│  text          │  TEXT         │  transcript line content       │
│  timestamp     │  DATETIME     │  wall-clock or audio-relative  │
└────────────────┴───────────────┴────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  TABLE: call_summaries                                          │
├────────────────┬───────────────┬────────────────────────────────┤
│  Column        │  Type         │  Notes                         │
├────────────────┼───────────────┼────────────────────────────────┤
│  call_id       │  TEXT PK      │  FK → calls.id                 │
│  issue         │  TEXT         │  what the customer reported    │
│  resolution    │  TEXT         │  what was done to resolve it   │
│  follow_up     │  TEXT         │  any action required after     │
│  sentiment_label│ TEXT         │  positive|neutral|negative     │
│  created_at    │  DATETIME     │  summary generation time       │
└────────────────┴───────────────┴────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  TABLE: call_scores                                             │
├─────────────────┬──────────────┬────────────────────────────────┤
│  Column         │  Type        │  Notes                         │
├─────────────────┼──────────────┼────────────────────────────────┤
│  call_id        │  TEXT PK     │  FK → calls.id                 │
│  empathy        │  INTEGER     │  1-10, agent empathy           │
│  resolution     │  INTEGER     │  1-10, issue resolution        │
│  professionalism│  INTEGER     │  1-10, tone and conduct        │
│  overall        │  INTEGER     │  1-10, composite score         │
│  notes          │  TEXT        │  one-sentence LLM summary      │
│  scored_at      │  DATETIME    │  async scoring completion time │
└─────────────────┴──────────────┴────────────────────────────────┘
```

**Entity Relationship:**

```
  calls 1──────────────────── * transcripts
    │                               (call_id FK)
    │
    ├──────────────────────── 0..1 call_summaries
    │                               (call_id PK/FK)
    │
    └──────────────────────── 0..1 call_scores
                                    (call_id PK/FK)

  agents ──── (current_call_id references calls.id, not a hard FK)
```

**Write strategy:** All writes go through `store.UpsertCall()` and `store.UpsertAgent()`, which use `INSERT ... ON CONFLICT(id) DO UPDATE`. This is idempotent — calling upsert twice with the same state is safe and produces no duplicates.

---

## REST API Reference

All endpoints return `Content-Type: application/json`.

### Health Check

```
GET /health

Response 200:
{
  "status": "ok",
  "service": "cortex-cc"
}
```

### Calls

```
GET /api/calls

Returns all non-terminal calls (queued + active + on_hold).

Response 200: array of Call objects
[
  {
    "id": "C-a1b2c3",
    "caller_number": "416-234-5678",
    "caller_name": "Michael Johnson",
    "agent_id": "uuid-...",
    "queue_name": "Billing",
    "status": "active",
    "wait_seconds": 18,
    "talk_seconds": 94,
    "sentiment": -0.41,
    "sla_breached": false,
    "started_at": "2026-07-28T14:32:01Z",
    "flagged": false
  },
  ...
]
```

```
GET /api/calls/{id}/transcript

Response 200: array of Transcript objects
[
  {
    "id": "uuid-...",
    "call_id": "C-a1b2c3",
    "speaker": "customer",
    "text": "Hi, I was charged twice this month.",
    "timestamp": "2026-07-28T14:32:07Z"
  },
  ...
]
```

```
GET /api/calls/{id}/summary

Response 200:
{
  "call_id": "C-a1b2c3",
  "issue": "Double billing charge in July",
  "resolution": "Credit issued for duplicate charge",
  "follow_up": "Confirm credit appears within 24h",
  "sentiment_label": "negative",
  "created_at": "2026-07-28T14:55:12Z"
}

Response 404: { "error": "summary not yet available" }
```

```
GET /api/calls/{id}/score

Returns the QA score for a completed call. Scores are generated asynchronously
by Llama 3.1 within seconds of call completion and stored in SQLite.

Response 200:
{
  "call_id": "C-a1b2c3",
  "empathy": 9,
  "resolution": 8,
  "professionalism": 9,
  "overall": 9,
  "notes": "Agent handled billing dispute with empathy and resolved efficiently.",
  "scored_at": "2026-07-28T15:03:42Z"
}

Response 404: { "error": "score not yet available — QA scoring happens asynchronously after call completion" }
```

### Agents

```
GET /api/agents

Response 200: array of Agent objects
[
  {
    "id": "uuid-...",
    "name": "Sarah Mitchell",
    "status": "busy",
    "current_call_id": "C-a1b2c3",
    "calls_handled": 7,
    "avg_handle_time_seconds": 183.4,
    "skills": ["Sales", "Billing"]
  },
  ...
]
```

### Queues

```
GET /api/queues

Response 200: array of QueueStats objects
[
  {
    "queue_name": "Billing",
    "active_calls": 3,
    "waiting_calls": 2,
    "available_agents": 2,
    "avg_wait_seconds": 45.2,
    "sla_breach_count": 1,
    "abandon_rate": 0.0
  },
  ...
]
```

### AI Copilot Chat

```
POST /api/chat
Content-Type: application/json
{ "message": "Which agents are struggling right now?" }

Response 200:
{
  "reply": "Based on current data: James Okafor has handled 14 calls
            today with an average handle time of 312 seconds, and his
            last 3 calls scored below -0.4 sentiment. Consider a
            10-minute break before the 3pm peak."
}
```

```
POST /api/chat/reset

Response 200: { "status": "conversation reset" }
```

### Transcription (Whisper)

```
POST /api/transcribe
Content-Type: multipart/form-data

Fields:
  audio    (file, required) — wav/mp3/m4a/ogg/flac/webm/mp4, max 32 MB
  call_id  (string, optional) — if set, segments are stored in SQLite
  speaker  (string, optional) — "agent" or "customer", default "AGENT"

Response 200:
{
  "text": "My internet has been down for three days and I need this fixed.",
  "segments": [
    { "id": 0, "start": 0.0,  "end": 2.4, "text": "My internet has been down" },
    { "id": 1, "start": 2.4,  "end": 5.1, "text": "for three days" },
    { "id": 2, "start": 5.1,  "end": 7.8, "text": "and I need this fixed." }
  ],
  "language": "en",
  "duration": 7.8,
  "elapsed": 1.43,
  "call_id": "C-a1b2c3",
  "stored": 3
}

Response 503: { "error": "whisper service not configured" }
  — Whisper container was not reachable at startup.
```

---

## WebSocket Events Reference

Connect to `ws://localhost:8080/ws`. All messages are JSON.

### Event Shape

```json
{
  "type": "event_type_string",
  "payload": { ... }
}
```

### Event Types

| Event | Trigger | Payload Fields |
|---|---|---|
| `call_queued` | New call arrives | `id, caller_name, caller_number, queue_name, status, started_at` |
| `call_answered` | Agent picks up | `id, agent_id, status: "active"` |
| `call_updated` | Every 2s tick | Full call object with updated timers + sentiment |
| `call_routed` | Supervisor routes via LLM/API | `id, agent_id, status: "active"` |
| `call_flagged` | Call flagged for QA | `id, flagged: true, flag_reason` |
| `call_completed` | Call ends normally | `id, status: "completed", ended_at` |
| `call_abandoned` | Customer hangs up in queue | `id, status: "abandoned", ended_at` |
| `sla_breached` | Wait > 120s | `id, caller_name, queue_name, wait_seconds` |
| `sentiment_update` | Every 60s of active talk | `id, caller_name, sentiment` |
| `transcript_line` | Every 6s (simulated) | `call_id, speaker, text, timestamp` |
| `alert` (monitor) | Every 60s if anomaly | `source, level, title, detail, ts` |
| `alert` (ai_advisory) | After anomaly if Ollama up | `source: "ai_advisory", detail: "..."` |
| `agent_assist` | Customer speaks on active call | `call_id, agent_id, queue, trigger, suggestion` |
| `call_scored` | QA score ready after call ends | `call_id, empathy, resolution, professionalism, overall, notes, scored_at` |

### Example: Receiving Agent Assist

```javascript
ws.onmessage = ({ data }) => {
  const evt = JSON.parse(data);
  if (evt.type === 'agent_assist') {
    console.log(evt.payload);
    // {
    //   call_id: "C-a1b2c3",
    //   agent_id: "uuid-...",
    //   queue: "Support",
    //   trigger: "I've already restarted it three times",
    //   suggestion: "I completely understand your frustration —
    //                let me escalate this to our Tier 2 team right now
    //                and ensure they call you back within the hour."
    // }
  }
};
```

---

## MCP Tools Reference

The 7 tools are registered with the MCP server (for external MCP clients) and with the Ollama chat loop (for LLM tool-calling). Both consumers use the same `ToolRegistry.Execute()` method.

### Tool: `get_queue_status`

Returns live health of all call queues.

```
Parameters: none

Response JSON:
{
  "queues": [
    {
      "queue_name": "Billing",
      "active_calls": 3,
      "waiting_calls": 2,
      "avg_wait_seconds": 45.2,
      "sla_breaches": 1,
      "longest_wait_seconds": 134
    },
    ...
  ],
  "total_active_calls": 8
}
```

---

### Tool: `get_agent_states`

Returns all agents sorted alphabetically with full status.

```
Parameters: none

Response JSON:
{
  "agents": [
    {
      "id": "uuid-...",
      "name": "Aiko Tanaka",
      "status": "available",
      "calls_handled_today": 5,
      "avg_handle_time_seconds": 201.4,
      "skills": ["Support", "Billing"]
    },
    ...
  ],
  "available_count": 4,
  "total": 10
}
```

---

### Tool: `get_call_transcript`

Returns the full ordered transcript for a call.

```
Parameters:
  call_id (string, required) — e.g. "C-a1b2c3"

Response JSON:
{
  "call_id": "C-a1b2c3",
  "transcript": [
    {
      "speaker": "agent",
      "text": "Thank you for calling Billing, how can I assist you?",
      "timestamp": "2026-07-28T14:32:01Z"
    },
    {
      "speaker": "customer",
      "text": "I was charged twice this month and I need that fixed.",
      "timestamp": "2026-07-28T14:32:07Z"
    }
  ],
  "line_count": 2
}
```

---

### Tool: `get_sentiment_report`

Returns sentiment scores for all active calls, sorted most-negative first.

```
Parameters: none

Response JSON:
{
  "active_calls": [
    {
      "call_id": "C-b2c3d4",
      "caller_name": "Sophia Williams",
      "queue": "Billing",
      "sentiment": -0.72,
      "label": "negative"
    },
    ...
  ],
  "average_sentiment": -0.21,
  "negative_calls": 3
}
```

---

### Tool: `route_call`

Transfers a queued or active call to a specific agent. Agent must be available.

```
Parameters:
  call_id  (string, required)
  agent_id (string, required)

Response JSON (success):
{
  "success": true,
  "message": "call C-a1b2c3 has been transferred to agent uuid-..."
}

Response JSON (failure):
{
  "success": false,
  "message": "could not route call C-a1b2c3 to agent uuid-...
              — agent may be unavailable or call not found"
}
```

---

### Tool: `flag_call`

Marks a call for QA review with a reason string.

```
Parameters:
  call_id (string, required)
  reason  (string, required) — e.g. "angry customer", "billing dispute"

Response JSON:
{
  "success": true,
  "message": "call C-a1b2c3 flagged for QA review: angry customer"
}
```

---

### Tool: `summarize_call`

Builds a structured prompt from a call's transcript for LLM summarisation.

```
Parameters:
  call_id (string, required)

Response JSON:
{
  "call_id": "C-a1b2c3",
  "transcript": "[agent]: Thank you for calling...\n[customer]: ...\n",
  "instruction": "Generate a JSON summary with fields: issue, resolution,
                  follow_up, sentiment_label (positive/neutral/negative)"
}
```

The LLM then generates the actual summary from this payload. The result can be stored via `store.InsertSummary()`.

---

## Tech Stack

### Why These Exact Tools

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Layer        │  Technology              │  The Reason                  │
├───────────────┼──────────────────────────┼──────────────────────────────┤
│  Language     │  Go 1.25                 │  Goroutines handle the 3     │
│               │                          │  concurrent tick loops +     │
│               │                          │  websocket hub cleanly.      │
│               │                          │  Single binary deployment.   │
├───────────────┼──────────────────────────┼──────────────────────────────┤
│  MCP          │  mark3labs/mcp-go        │  The only production-grade   │
│               │                          │  MCP server SDK for Go.      │
├───────────────┼──────────────────────────┼──────────────────────────────┤
│  LLM runtime  │  Ollama + Llama 3.1:8b   │  Free, runs on CPU, natively │
│               │                          │  supports tool-calling.      │
│               │                          │  8B fits in 8 GB RAM.        │
├───────────────┼──────────────────────────┼──────────────────────────────┤
│  Database     │  modernc.org/sqlite      │  Pure Go SQLite — no CGO,    │
│               │                          │  no gcc needed, cross-       │
│               │                          │  compiles to any platform.   │
├───────────────┼──────────────────────────┼──────────────────────────────┤
│  WebSocket    │  gorilla/websocket       │  Battle-tested, used by      │
│               │                          │  Kubernetes, Docker, etc.    │
├───────────────┼──────────────────────────┼──────────────────────────────┤
│  STT          │  OpenAI Whisper (base)   │  Apache-2.0, runs on CPU,    │
│               │                          │  74 MB model, good English   │
│               │                          │  accuracy for call audio.    │
├───────────────┼──────────────────────────┼──────────────────────────────┤
│  NLP          │  DistilBERT (SST-2)      │  66M params, 97% of BERT     │
│               │                          │  accuracy, fast on CPU.      │
│               │                          │  Pre-downloaded at Docker    │
│               │                          │  build time — no HF API.    │
├───────────────┼──────────────────────────┼──────────────────────────────┤
│  Frontend     │  HTML + Tailwind CSS     │  No build step. Single file. │
│               │                          │  Served by Go directly.      │
├───────────────┼──────────────────────────┼──────────────────────────────┤
│  Deploy       │  Docker + Compose        │  One command. Self-contained. │
└───────────────┴──────────────────────────┴──────────────────────────────┘
```

### Dependencies (go.mod)

```
github.com/google/uuid         v1.6.0   — call and transcript IDs
github.com/gorilla/websocket   v1.5.3   — WebSocket hub
github.com/mark3labs/mcp-go    v0.57.0  — MCP stdio server
modernc.org/sqlite             v1.54.0  — embedded database (pure Go)
```

Zero heavyweight framework dependencies. Standard library does the rest.

---

## Project Structure

```
cortex-cc/
│
├── cmd/
│   ├── server/
│   │   └── main.go              Entry point. Wires all components in order:
│   │                            store → engine → hub → llm → mcp → assist
│   │                            → qa → monitor → transcriber → server
│   └── mcp-server/
│       └── main.go              Standalone MCP stdio binary. HTTPExecutor
│                                calls cortex-cc REST API. Works with Cursor,
│                                VS Code Copilot, Claude Desktop, etc.
│
├── internal/
│   ├── config/
│   │   └── config.go            Reads 6 env vars with sane defaults.
│   │
│   ├── models/
│   │   └── models.go            All domain types: Call, Agent, Transcript,
│   │                            CallSummary, QueueStats, Event.
│   │                            Shared by every package.
│   │
│   ├── store/
│   │   └── store.go             SQLite layer. 4 tables, idempotent upserts,
│   │                            all queries in plain SQL. No ORM.
│   │
│   ├── engine/
│   │   ├── engine.go            Core engine struct, Start(), tick loop,
│   │   │                        RouteCall(), FlagCall(), UpdateSentiment().
│   │   ├── generator.go         Call generator goroutine (every 8s).
│   │   ├── transcript.go        Transcript generator goroutine (every 6s).
│   │   │                        Also fires OnCustomerLine hook.
│   │   ├── util.go              randN(), randFloat(), clamp(), hasSkill().
│   │   └── engine_test.go       8 unit tests. :memory: SQLite, race-safe.
│   │
│   ├── websocket/
│   │   └── hub.go               Hub with fan-out Broadcast(). Slow clients
│   │                            are dropped (select/default) — no blocking.
│   │
│   ├── mcp/
│   │   ├── tools.go             ToolRegistry with Definitions() + Execute().
│   │   │                        7 tools. Shared by Ollama loop + MCP server.
│   │   ├── server.go            MCP stdio server (mark3labs/mcp-go).
│   │   └── tools_test.go        12 unit tests: counts, names, JSON schemas,
│   │                            Execute dispatch, error paths.
│   │
│   ├── llm/
│   │   ├── client.go            Ollama HTTP client. POST /api/chat, 90s timeout.
│   │   └── loop.go              Agentic loop. Multi-turn history. Max 5 rounds.
│   │                            System prompt baked in.
│   │
│   ├── monitor/
│   │   └── monitor.go           60s poll. detectAnomalies() checks queue,
│   │                            sentiment, agent availability. Calls LLM
│   │                            for advisory if anomalies found.
│   │
│   ├── sentiment/
│   │   └── client.go            HTTP client for DistilBERT service.
│   │                            Score(text) → float64, Ping() → error.
│   │
│   ├── transcriber/
│   │   └── client.go            HTTP client for Whisper service.
│   │                            TranscribeBytes(audio, filename) → *Result.
│   │
│   ├── assist/
│   │   └── service.go           Agent assist. Rate-limited (40s/call).
│   │                            Fetches last 6 lines, calls Ollama, broadcasts.
│   │
│   ├── qa/
│   │   └── scorer.go            Post-call QA scorer. Async goroutine per call.
│   │                            OneShot LLM call, JSON brace extraction,
│   │                            clamp 1-10, store + WebSocket broadcast.
│   │
│   └── server/
│       └── server.go            HTTP handlers for all 11 routes.
│                                Multipart audio parsing for /api/transcribe.
│
├── web/
│   └── index.html               Single-file supervisor dashboard.
│                                WebSocket client, agent poll, AI chat,
│                                assist cards, alert banners.
│
├── whisper/
│   ├── service.py               FastAPI Whisper STT service (port 8001).
│   ├── requirements.txt         openai-whisper, fastapi, uvicorn
│   └── Dockerfile               python:3.11-slim + ffmpeg + whisper
│
├── sentiment/
│   ├── service.py               FastAPI DistilBERT service (port 5001).
│   ├── requirements.txt         transformers, fastapi, uvicorn, torch
│   └── Dockerfile               Model pre-downloaded at build time.
│
├── Makefile                     run, build, test, lint, docker-up/down/logs
├── DEMO.md                      Step-by-step hiring-manager demo script
├── Dockerfile                   Multi-stage Go build → minimal final image
├── docker-compose.yml           4 services: cortex, ollama, whisper, sentiment
└── README.md                    This file
```

---

## Installation & Quick Start

### Option A — Local (fastest)

**Prerequisites:** Go 1.25+, Ollama installed and running.

```bash
# 1. Pull the LLM model (one-time, ~4.7 GB)
ollama pull llama3.1:8b

# 2. Clone
git clone https://github.com/abhiramhatwar/cortex-cc.git
cd cortex-cc

# 3. Run (SQLite is created automatically, engine starts immediately)
make run
# or: go run ./cmd/server

# 4. Open dashboard
open http://localhost:8080
```

The call engine starts automatically. Within 10 seconds you'll see agents and calls populating the dashboard.

**Note:** Without the sentiment/whisper services running, cortex-cc degrades gracefully:
- Sentiment falls back to random drift (dashboard still shows bars)
- `/api/transcribe` returns `503 Service Unavailable`
- All other features work normally

---

### Option B — Docker Compose (full stack)

**Prerequisites:** Docker + Docker Compose, 8 GB free RAM.

```bash
git clone https://github.com/abhiramhatwar/cortex-cc.git
cd cortex-cc

# Start all 4 services
make docker-up

# Pull the LLM model into the Ollama container (one-time, ~4.7 GB)
make docker-pull-model

# Open dashboard
open http://localhost:8080
```

**What starts:**

```
┌────────────┬─────────────────────────────────────┬───────┬──────────────────┐
│  Container │  Role                               │  Port │  Image           │
├────────────┼─────────────────────────────────────┼───────┼──────────────────┤
│  ollama    │  Local LLM runtime (Llama 3.1:8b)   │ 11434 │  ollama/ollama   │
│  sentiment │  DistilBERT sentiment service       │  5001 │  built from ./sentiment│
│  whisper   │  Whisper STT service                │  8001 │  built from ./whisper  │
│  cortex    │  Go API + dashboard                 │  8080 │  built from ./Dockerfile│
└────────────┴─────────────────────────────────────┴───────┴──────────────────┘
```

**Useful Docker commands:**

```bash
make docker-logs      # tail all container logs
make docker-down      # stop and remove all containers
docker compose ps     # check container health
```

---

### Option C — Binary Build

```bash
make build
# Produces: bin/cortex-cc

OLLAMA_URL=http://myserver:11434 ./bin/cortex-cc
```

---

## Configuration

All configuration is via environment variables. Every variable has a working default.

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `DB_PATH` | `./cortex.db` | SQLite database file path |
| `OLLAMA_URL` | `http://localhost:11434` | Ollama server base URL |
| `OLLAMA_MODEL` | `llama3.1:8b` | Model name for all LLM calls |
| `WHISPER_URL` | `http://localhost:8001` | Whisper STT microservice URL |
| `SENTIMENT_URL` | `http://localhost:5001` | DistilBERT sentiment service URL |
| `HF_TOKEN` | *(empty)* | HuggingFace token — only needed for gated models |

**Whisper-specific** (set on the whisper container):

| Variable | Default | Options |
|---|---|---|
| `WHISPER_MODEL` | `base` | `tiny`, `base`, `small`, `medium`, `large` |
| `WHISPER_DEVICE` | `cpu` | `cpu`, `cuda` |

**Sentiment-specific** (set on the sentiment container):

| Variable | Default | Options |
|---|---|---|
| `SENTIMENT_MODEL` | `distilbert-base-uncased-finetuned-sst-2-english` | Any HF classifier |
| `SENTIMENT_DEVICE` | `-1` | `-1` (CPU), `0` (first GPU) |

---

## Running Tests

```bash
# Run all tests with race detector
make test

# Run without race detector (faster)
go test ./...

# Run a specific package
go test ./internal/engine/...
go test ./internal/mcp/...

# Verbose output
go test -v -count=1 ./...
```

### Test Coverage

| Package | Tests | What They Cover |
|---|---|---|
| `internal/engine` | 8 tests | Agent seeding, call operations, SLA, sentiment hook, engine lifecycle |
| `internal/mcp` | 12 tests | Tool definitions count, names, JSON schemas, Execute dispatch, error paths |

All tests use **SQLite in-memory** (`:memory:`) — no files created, no cleanup needed. The engine tests use a `injectCall()` helper to insert test calls directly into the engine map, bypassing the generator goroutine. All tests are race-safe (`-race` passes).

```
go test -race -count=1 ./...
ok      github.com/abhiram/cortex-cc/internal/engine    0.312s
ok      github.com/abhiram/cortex-cc/internal/mcp       0.284s
```

---

## Makefile Reference

```bash
make run              # go run ./cmd/server
make build            # compile to bin/cortex-cc (stripped binary)
make build-mcp        # compile standalone MCP binary to bin/mcp-server
make build-all        # build both cortex-cc and mcp-server
make test             # go test -race -count=1 ./...
make test-short       # go test -short -count=1 ./...
make lint             # go vet ./... + staticcheck (if installed)
make tidy             # go mod tidy
make docker-up        # docker compose up --build -d
make docker-down      # docker compose down
make docker-logs      # docker compose logs -f
make docker-pull-model # pull llama3.1:8b into ollama container
make clean            # rm -rf bin/
make help             # list all targets
```

---

## Security & Compliance

### Data Residency

```
┌───────────────────────────────────────────────────────────┐
│              What happens to sensitive call data?         │
├──────────────────────────────┬────────────────────────────┤
│  Caller PII (name, number)   │  SQLite on your disk only  │
│  Call recordings             │  Whisper processes locally  │
│  Transcript content          │  SQLite on your disk only  │
│  Sentiment scores            │  DistilBERT runs locally    │
│  LLM prompts and replies     │  Ollama runs locally        │
│                              │                            │
│  External API calls made:    │  ZERO                      │
└──────────────────────────────┴────────────────────────────┘
```

### Compliance Readiness

| Regulation | Requirement Addressed |
|---|---|
| **HIPAA** | No PHI transmitted outside your network. Ollama, Whisper, DistilBERT all on-prem. |
| **PCI-DSS** | No payment card data in scope for external transfer. |
| **GDPR** | Data never leaves your jurisdiction. You control the SQLite file. |
| **FedRAMP** | Self-hosted stack — auditable, no third-party cloud dependency. |
| **SOX** | Audit trail in SQLite. Call flags and summaries stored locally. |

### Network Boundaries

```
  Your Network
  ┌─────────────────────────────────────────────────────┐
  │                                                     │
  │  Browser → :8080 (cortex-cc)                        │
  │  cortex-cc → :11434 (ollama)     ← all internal     │
  │  cortex-cc → :5001 (sentiment)   ← all internal     │
  │  cortex-cc → :8001 (whisper)     ← all internal     │
  │                                                     │
  │  ══════════════ Internet boundary ═══════════════   │
  │                                                     │
  │  Nothing crosses this line.                         │
  └─────────────────────────────────────────────────────┘
```

---

## Competitive Analysis

### What Exists Today

```
               Features / Compliance
                         │
    High ────────────────┼──────────────────── Low
                         │
             cortex-cc   │
               ●         │
               │         │
               │         │      RingCentral
               │         │         ●
               │         │
               │         │
               │         │      Mitel CX 2.0
               │         │         ●
               │         │
  On-Prem ─────┼─────────┼────────────────── Cloud
```

### Gap Mitel Cannot Close Without cortex-cc Architecture

1. **MCP Support** — Mitel has no MCP integration. cortex-cc is plug-and-play with any MCP client.
2. **On-Prem AI** — Talkative AI is cloud-only. There is no on-prem AI option in Mitel's current roadmap.
3. **Open Model Choice** — cortex-cc can swap Llama 3.1 for any Ollama-compatible model (Mistral, Gemma, Phi-3) without changing a line of code. Just change `OLLAMA_MODEL`.
4. **Cost** — cortex-cc runs on existing hardware. No per-agent AI licensing cost.

---

## Commit History

27 commits built over 20 days. Each day targeted a specific system:

| Days | What Was Built |
|---|---|
| 1–3 | Project scaffold, domain models, SQLite store, Docker setup |
| 4–6 | Call engine goroutines, WebSocket hub, REST API |
| 7–9 | MCP server (7 tools), Ollama client, agentic tool-calling loop |
| 10–12 | `/api/chat` endpoint, supervisor dashboard, proactive monitor |
| 13–15 | Whisper STT microservice, Go transcriber client, `/api/transcribe` |
| 16–17 | HuggingFace DistilBERT sentiment service, Go client, engine wiring |
| 18–19 | Agent assist service, `OnCustomerLine` hook, dashboard assist panel |
| 20 | Engine unit tests, MCP tools tests, Makefile, DEMO.md, README |

---

## License

MIT — built by [Abhiram Hatwar](https://github.com/abhiramhatwar)

---

*cortex-cc demonstrates: MCP tool server design, agentic LLM loops, real-time Go concurrency patterns, on-prem AI microservices, and WebSocket-driven dashboards — all without a single paid API call.*
