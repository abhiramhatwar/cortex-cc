package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/abhiram/cortex-cc/internal/config"
	"github.com/abhiram/cortex-cc/internal/engine"
	"github.com/abhiram/cortex-cc/internal/server"
	"github.com/abhiram/cortex-cc/internal/store"
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

	srv := server.New(cfg, st, eng, hub)
	log.Fatal(srv.Start())
}
