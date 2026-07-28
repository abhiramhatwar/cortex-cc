package qa

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"cortex-cc/internal/llm"
	"cortex-cc/internal/models"
	"cortex-cc/internal/store"
	ws "cortex-cc/internal/websocket"
)

const systemPrompt = `You are a contact center QA analyst. Score the agent's performance on this completed call.
Respond ONLY with a valid JSON object — no markdown, no explanation. Use this exact schema:
{
  "empathy": <integer 1-10>,
  "resolution": <integer 1-10>,
  "professionalism": <integer 1-10>,
  "overall": <integer 1-10>,
  "notes": "<one concise sentence about the agent's performance>"
}
Scoring guide: 1-4 = poor, 5-7 = acceptable, 8-10 = excellent.`

// Scorer scores completed calls using a local LLM and stores the result.
type Scorer struct {
	llm   *llm.Client
	store *store.Store
	hub   *ws.Hub
}

func New(lc *llm.Client, st *store.Store, hub *ws.Hub) *Scorer {
	return &Scorer{llm: lc, store: st, hub: hub}
}

// ScoreCall is safe to use as engine.OnCallCompleted — launches async goroutine.
func (s *Scorer) ScoreCall(call *models.Call) {
	go s.scoreAsync(call)
}

func (s *Scorer) scoreAsync(call *models.Call) {
	lines, err := s.store.GetTranscriptByCallID(call.ID)
	if err != nil || len(lines) == 0 {
		return // no transcript — skip quietly
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Call ID: %s | Queue: %s | Talk time: %ds | Sentiment: %.2f\n\nTranscript:\n",
		call.ID, call.QueueName, call.TalkSeconds, call.Sentiment)
	for _, l := range lines {
		fmt.Fprintf(&sb, "[%s]: %s\n", l.Speaker, l.Text)
	}

	raw, err := s.llm.OneShot(systemPrompt, sb.String())
	if err != nil {
		log.Printf("qa: llm error for call %s: %v", call.ID, err)
		return
	}

	sc, err := parseScore(call.ID, raw)
	if err != nil {
		log.Printf("qa: parse error for call %s: %v — raw: %q", call.ID, err, raw)
		return
	}

	if err := s.store.InsertCallScore(sc); err != nil {
		log.Printf("qa: store error for call %s: %v", call.ID, err)
		return
	}

	s.hub.Broadcast(&models.Event{Type: "call_scored", Payload: sc})
	log.Printf("qa: scored call %s — overall %d/10 (%s)", call.ID, sc.Overall, sc.Notes)
}

func parseScore(callID, raw string) (*models.CallScore, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found")
	}
	raw = raw[start : end+1]

	var s struct {
		Empathy         int    `json:"empathy"`
		Resolution      int    `json:"resolution"`
		Professionalism int    `json:"professionalism"`
		Overall         int    `json:"overall"`
		Notes           string `json:"notes"`
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, err
	}

	return &models.CallScore{
		CallID:          callID,
		Empathy:         clamp(s.Empathy, 1, 10),
		Resolution:      clamp(s.Resolution, 1, 10),
		Professionalism: clamp(s.Professionalism, 1, 10),
		Overall:         clamp(s.Overall, 1, 10),
		Notes:           s.Notes,
		ScoredAt:        time.Now(),
	}, nil
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
