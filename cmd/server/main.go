package main

import (
	"log"

	"github.com/abhiram/cortex-cc/internal/config"
	"github.com/abhiram/cortex-cc/internal/server"
	"github.com/abhiram/cortex-cc/internal/store"
)

func main() {
	cfg := config.Load()

	st, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("store init: %v", err)
	}
	defer st.Close()

	srv := server.New(cfg, st)
	if err := srv.Start(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
