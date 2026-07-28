package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/abhiramhatwar/cortex-cc/internal/assist"
	"github.com/abhiramhatwar/cortex-cc/internal/config"
	"github.com/abhiramhatwar/cortex-cc/internal/engine"
	"github.com/abhiramhatwar/cortex-cc/internal/llm"
	"github.com/abhiramhatwar/cortex-cc/internal/mcp"
	"github.com/abhiramhatwar/cortex-cc/internal/monitor"
	"github.com/abhiramhatwar/cortex-cc/internal/sentiment"
	"github.com/abhiramhatwar/cortex-cc/internal/server"
	"github.com/abhiramhatwar/cortex-cc/internal/store"
	"github.com/abhiramhatwar/cortex-cc/internal/transcriber"
	ws "github.com/abhiramhatwar/cortex-cc/internal/websocket"
)

func main() {
	cfg := config.Load()

	st, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	// on-prem HuggingFace sentiment service (optional — degrades gracefully)
	sc := sentiment.New(cfg.SentimentURL)
	if err := sc.Ping(); err != nil {
		log.Printf("warning: sentiment service not reachable (%v) — using random drift", err)
		sc = nil
	}

	eng := engine.New(st, sc)
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

	// agent assist: generates real-time response suggestions for agents on customer lines
	assistSvc := assist.New(llmClient, hub, st)
	eng.OnCustomerLine = assistSvc.ProcessLine

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
