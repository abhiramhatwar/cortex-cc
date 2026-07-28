package assist

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/abhiram/cortex-cc/internal/llm"
	"github.com/abhiram/cortex-cc/internal/models"
	"github.com/abhiram/cortex-cc/internal/store"
	ws "github.com/abhiram/cortex-cc/internal/websocket"
)

const (
	// cooldown prevents flooding the LLM with suggestions for the same call.
	cooldown = 40 * time.Second
)

const systemPrompt = `You are an agent assist AI embedded in a contact center platform.
When a customer says something during a live call, you suggest ONE concise response the agent should say next.

Rules:
- Maximum 2 sentences
- Professional and empathetic tone
- Directly address what the customer just said
- Do not use hollow filler phrases like "Great question!", "Absolutely!", or "Of course!"
- Return ONLY the suggested agent response — no explanation, no prefix, no quotes`

// Service generates real-time response suggestions for agents during active calls.
// It is triggered by each customer transcript line and rate-limited per call.
type Service struct {
	client *llm.Client
	hub    *ws.Hub
	store  *store.Store

	mu      sync.Mutex
	lastSug map[string]time.Time // callID → last suggestion timestamp
}

func New(client *llm.Client, hub *ws.Hub, st *store.Store) *Service {
	return &Service{
		client:  client,
		hub:     hub,
		store:   st,
		lastSug: make(map[string]time.Time),
	}
}

// ProcessLine is called whenever a customer says something on an active call.
// It generates an agent suggestion and broadcasts it via WebSocket.
// Rate-limited: at most one suggestion per call per cooldown window.
func (s *Service) ProcessLine(call *models.Call, customerText string) {
	s.mu.Lock()
	if time.Since(s.lastSug[call.ID]) < cooldown {
		s.mu.Unlock()
		return
	}
	s.lastSug[call.ID] = time.Now()
	s.mu.Unlock()

	// Fetch last 6 transcript lines for context
	lines, _ := s.store.GetTranscriptByCallID(call.ID)
	var ctxBuf strings.Builder
	start := len(lines) - 6
	if start < 0 {
		start = 0
	}
	for _, l := range lines[start:] {
		ctxBuf.WriteString(fmt.Sprintf("[%s]: %s\n", strings.ToUpper(l.Speaker), l.Text))
	}

	prompt := fmt.Sprintf(
		"Queue: %s\n\nRecent conversation:\n%s\nCustomer just said: %q\n\nSuggested agent response:",
		call.QueueName, ctxBuf.String(), customerText,
	)

	msgs := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}

	resp, err := s.client.Chat(msgs, nil)
	if err != nil {
		log.Printf("assist: llm error for call %s: %v", call.ID, err)
		return
	}

	suggestion := strings.TrimSpace(resp.Content)
	if suggestion == "" {
		return
	}

	log.Printf("assist: [%s] %s → %q", call.ID, call.QueueName, suggestion)

	s.hub.Broadcast(&models.Event{
		Type: "agent_assist",
		Payload: map[string]any{
			"call_id":    call.ID,
			"agent_id":   call.AgentID,
			"queue":      call.QueueName,
			"trigger":    customerText,
			"suggestion": suggestion,
		},
	})
}
