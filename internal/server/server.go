package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"io"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"cortex-cc/internal/config"
	"cortex-cc/internal/engine"
	"cortex-cc/internal/llm"
	"cortex-cc/internal/metrics"
	"cortex-cc/internal/models"
	"cortex-cc/internal/store"
	"cortex-cc/internal/transcriber"
	ws "cortex-cc/internal/websocket"
)

type Server struct {
	cfg         *config.Config
	store       *store.Store
	engine      *engine.Engine
	hub         *ws.Hub
	loop        *llm.Loop
	transcriber *transcriber.Client
	mux         *http.ServeMux
}

func New(cfg *config.Config, st *store.Store, eng *engine.Engine, hub *ws.Hub, loop *llm.Loop, tr *transcriber.Client) *Server {
	s := &Server{cfg: cfg, store: st, engine: eng, hub: hub, loop: loop, transcriber: tr, mux: http.NewServeMux()}

	reg := prometheus.NewRegistry()
	reg.MustRegister(metrics.NewCollector(eng, st))
	s.mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /ws", s.hub.ServeWS)
	s.mux.HandleFunc("POST /api/chat", s.handleChat)
	s.mux.HandleFunc("POST /api/chat/reset", s.handleChatReset)
	s.mux.HandleFunc("POST /api/transcribe", s.handleTranscribe)
	s.mux.HandleFunc("GET /api/calls", s.handleGetCalls)
	s.mux.HandleFunc("GET /api/agents", s.handleGetAgents)
	s.mux.HandleFunc("GET /api/queues", s.handleGetQueues)
	s.mux.HandleFunc("GET /api/calls/{id}/transcript", s.handleGetTranscript)
	s.mux.HandleFunc("GET /api/calls/{id}/summary", s.handleGetSummary)
	s.mux.HandleFunc("GET /api/calls/{id}/score", s.handleGetScore)
	s.mux.HandleFunc("GET /api/routing/best-agent", s.handleBestAgent)
	s.mux.HandleFunc("POST /api/calls/{id}/flag", s.handleFlagCall)
	s.mux.HandleFunc("POST /api/calls/{id}/route", s.handleRouteCall)
	s.mux.Handle("/", http.FileServer(http.Dir("./web")))
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

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}
	reply, err := s.loop.Chat(req.Message)
	if err != nil {
		log.Printf("chat error: %v", err)
		writeJSON(w, http.StatusOK, map[string]string{"reply": "AI unavailable — is Ollama running? " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reply": reply})
}

func (s *Server) handleChatReset(w http.ResponseWriter, r *http.Request) {
	s.loop.ResetHistory()
	writeJSON(w, http.StatusOK, map[string]string{"status": "conversation reset"})
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

func (s *Server) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if s.transcriber == nil {
		writeError(w, http.StatusServiceUnavailable, "whisper service not configured")
		return
	}

	// 32 MB max upload
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("audio")
	if err != nil {
		writeError(w, http.StatusBadRequest, "audio field is required")
		return
	}
	defer file.Close()

	audio, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read audio")
		return
	}

	callID := r.FormValue("call_id")
	speaker := r.FormValue("speaker")
	if speaker == "" {
		speaker = "AGENT"
	}

	result, err := s.transcriber.TranscribeBytes(audio, header.Filename)
	if err != nil {
		log.Printf("transcribe error: %v", err)
		writeError(w, http.StatusBadGateway, "whisper service error: "+err.Error())
		return
	}

	// If a call_id was provided, persist each segment as a transcript entry.
	stored := 0
	if callID != "" && len(result.Segments) > 0 {
		base := time.Now().Add(-time.Duration(result.Duration) * time.Second)
		for _, seg := range result.Segments {
			t := &models.Transcript{
				ID:        uuid.NewString(),
				CallID:    callID,
				Speaker:   speaker,
				Text:      seg.Text,
				Timestamp: base.Add(time.Duration(seg.Start*float64(time.Second))),
			}
			if err := s.store.InsertTranscript(t); err != nil {
				log.Printf("store transcript segment: %v", err)
			} else {
				stored++
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"text":      result.Text,
		"segments":  result.Segments,
		"language":  result.Language,
		"duration":  result.Duration,
		"elapsed":   result.Elapsed,
		"call_id":   callID,
		"stored":    stored,
	})
}

func (s *Server) handleGetScore(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	score, err := s.store.GetCallScore(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if score == nil {
		writeError(w, http.StatusNotFound, "score not yet available — QA scoring happens asynchronously after call completion")
		return
	}
	writeJSON(w, http.StatusOK, score)
}

func (s *Server) handleBestAgent(w http.ResponseWriter, r *http.Request) {
	queue := r.URL.Query().Get("queue")
	if queue == "" {
		writeError(w, http.StatusBadRequest, "queue parameter required: Sales, Billing, or Support")
		return
	}
	agent, reason := s.engine.BestAgentFor(queue)
	if agent == nil {
		writeJSON(w, http.StatusOK, map[string]any{"agent": nil, "reason": reason, "queue": queue})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent": agent, "reason": reason, "queue": queue})
}

func (s *Server) handleFlagCall(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if !s.engine.FlagCall(id, req.Reason) {
		writeError(w, http.StatusNotFound, "call not found: "+id)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("call %s flagged: %s", id, req.Reason),
	})
}

func (s *Server) handleRouteCall(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	if !s.engine.RouteCall(id, req.AgentID) {
		writeError(w, http.StatusBadRequest, "could not route call — agent may be unavailable or call not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("call %s routed to agent %s", id, req.AgentID),
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
