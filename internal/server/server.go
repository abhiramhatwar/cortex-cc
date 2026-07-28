package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/abhiram/cortex-cc/internal/config"
	"github.com/abhiram/cortex-cc/internal/engine"
	"github.com/abhiram/cortex-cc/internal/store"
	ws "github.com/abhiram/cortex-cc/internal/websocket"
)

type Server struct {
	cfg    *config.Config
	store  *store.Store
	engine *engine.Engine
	hub    *ws.Hub
	mux    *http.ServeMux
}

func New(cfg *config.Config, st *store.Store, eng *engine.Engine, hub *ws.Hub) *Server {
	s := &Server{cfg: cfg, store: st, engine: eng, hub: hub, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /ws", s.hub.ServeWS)
	s.mux.HandleFunc("GET /api/calls", s.handleGetCalls)
	s.mux.HandleFunc("GET /api/agents", s.handleGetAgents)
	s.mux.HandleFunc("GET /api/queues", s.handleGetQueues)
	s.mux.HandleFunc("GET /api/calls/{id}/transcript", s.handleGetTranscript)
	s.mux.HandleFunc("GET /api/calls/{id}/summary", s.handleGetSummary)
	s.mux.Handle("GET /", http.FileServer(http.Dir("./web")))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%s", s.cfg.Port)
	log.Printf("cortex-cc listening on %s", addr)
	return http.ListenAndServe(addr, s)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "cortex-cc"})
}

func (s *Server) handleGetCalls(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.GetActiveCalls())
}

func (s *Server) handleGetAgents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.GetAgents())
}

func (s *Server) handleGetQueues(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetQueueStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleGetTranscript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	transcripts, err := s.store.GetTranscriptByCallID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, transcripts)
}

func (s *Server) handleGetSummary(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	summary, err := s.store.GetSummaryByCallID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if summary == nil {
		writeError(w, http.StatusNotFound, "summary not yet available")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
