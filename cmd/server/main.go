package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/abhiram/cortex-cc/internal/config"
	"github.com/abhiram/cortex-cc/internal/engine"
	"github.com/abhiram/cortex-cc/internal/llm"
	"github.com/abhiram/cortex-cc/internal/mcp"
	"github.com/abhiram/cortex-cc/internal/monitor"
	"github.com/abhiram/cortex-cc/internal/server"
	"github.com/abhiram/cortex-cc/internal/store"
	"github.com/abhiram/cortex-cc/internal/transcriber"
	ws "github.com/abhiram/cortex-cc/internal/websocket"
)

func main() {
	cfg := config.Load()

	st, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	eng := engine.New(st)
	hub := ws.NewHub()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// start engine and fan events into the WebSocket hub
	eng.Start(ctx)
	go hub.Run(eng.Events)

	// wire up the AI copilot: Ollama client → tool registry → agentic loop
	llmClient := llm.NewClient(cfg.OllamaURL, cfg.OllamaModel)
	if err := llmClient.Ping(); err != nil {
		log.Printf("warning: ollama not reachable (%v) — AI copilot will be unavailable", err)
	}
	registry := mcp.NewToolRegistry(eng, st)
	loop := llm.NewLoop(llmClient, registry)

	// proactive anomaly monitor: polls every 60s, broadcasts alerts + AI advisories
	mon := monitor.New(registry, loop, hub)
	mon.Start(ctx)

	// on-prem Whisper transcription service (optional — degrades gracefully if not running)
	tr := transcriber.New(cfg.WhisperURL)
	if err := tr.Ping(); err != nil {
		log.Printf("warning: whisper service not reachable (%v) — /api/transcribe will be unavailable", err)
		tr = nil
	}

	srv := server.New(cfg, st, eng, hub, loop, tr)
	log.Fatal(srv.Start())
}
