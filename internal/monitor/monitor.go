package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cortex-cc/internal/llm"
	"cortex-cc/internal/mcp"
	"cortex-cc/internal/models"
	ws "cortex-cc/internal/websocket"
)

const pollInterval = 60 * time.Second

// Anomaly levels
const (
	LevelWarning  = "warning"
	LevelCritical = "critical"
)

// Monitor polls the contact center every 60s and autonomously fires alerts
// when it detects SLA breaches, negative sentiment spikes, or queue overloads.
// It uses the LLM to generate a human-readable summary of what it found.
type Monitor struct {
	registry *mcp.ToolRegistry
	loop     *llm.Loop
	hub      *ws.Hub
}

func New(registry *mcp.ToolRegistry, loop *llm.Loop, hub *ws.Hub) *Monitor {
	return &Monitor{registry: registry, loop: loop, hub: hub}
}

// Start begins autonomous monitoring in the background.
func (m *Monitor) Start(ctx context.Context) {
	go m.run(ctx)
	log.Printf("monitor: proactive anomaly detection started (interval: %s)", pollInterval)
}

func (m *Monitor) run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.poll(ctx)
		}
	}
}

func (m *Monitor) poll(ctx context.Context) {
	anomalies := m.detectAnomalies()
	if len(anomalies) == 0 {
		return
	}

	for _, a := range anomalies {
		log.Printf("monitor: anomaly detected [%s] %s", a.Level, a.Title)
		m.hub.Broadcast(&models.Event{
			Type: models.EventTypeAlert,
			Payload: map[string]any{
				"source":  "monitor",
				"level":   a.Level,
				"title":   a.Title,
				"detail":  a.Detail,
				"ts":      time.Now().Unix(),
			},
		})
	}

	// Ask the LLM to produce a consolidated advisory for the supervisor.
	if len(anomalies) > 0 {
		go m.generateAdvisory(anomalies)
	}
}

type anomaly struct {
	Level  string
	Title  string
	Detail string
}

func (m *Monitor) detectAnomalies() []anomaly {
	var out []anomaly

	// ── Queue health ──────────────────────────────────────────────────────────
	queueJSON, err := m.registry.Execute("get_queue_status", nil)
	if err == nil {
		var q struct {
			Queues []struct {
				Name           string  `json:"queue_name"`
				WaitingCalls   int     `json:"waiting_calls"`
				SLABreaches    int     `json:"sla_breaches"`
				LongestWait    int     `json:"longest_wait_seconds"`
				AvgWaitSeconds float64 `json:"avg_wait_seconds"`
			} `json:"queues"`
		}
		if json.Unmarshal([]byte(queueJSON), &q) == nil {
			for _, queue := range q.Queues {
				if queue.SLABreaches > 0 {
					level := LevelWarning
					if queue.SLABreaches >= 3 {
						level = LevelCritical
					}
					out = append(out, anomaly{
						Level:  level,
						Title:  fmt.Sprintf("SLA breach in %s queue", queue.Name),
						Detail: fmt.Sprintf("%d calls breached SLA (longest wait: %ds)", queue.SLABreaches, queue.LongestWait),
					})
				}
				if queue.WaitingCalls >= 5 {
					out = append(out, anomaly{
						Level:  LevelWarning,
						Title:  fmt.Sprintf("Queue overload in %s", queue.Name),
						Detail: fmt.Sprintf("%d calls waiting, avg wait %.0fs", queue.WaitingCalls, queue.AvgWaitSeconds),
					})
				}
			}
		}
	}

	// ── Sentiment ─────────────────────────────────────────────────────────────
	sentJSON, err := m.registry.Execute("get_sentiment_report", nil)
	if err == nil {
		var s struct {
			AverageSentiment float64 `json:"average_sentiment"`
			NegativeCalls    int     `json:"negative_calls"`
			ActiveCalls      []struct {
				CallerName string  `json:"caller_name"`
				Sentiment  float64 `json:"sentiment"`
				Queue      string  `json:"queue"`
			} `json:"active_calls"`
		}
		if json.Unmarshal([]byte(sentJSON), &s) == nil {
			if s.NegativeCalls >= 3 || s.AverageSentiment < -0.4 {
				level := LevelWarning
				if s.AverageSentiment < -0.6 {
					level = LevelCritical
				}
				out = append(out, anomaly{
					Level:  level,
					Title:  "Negative sentiment spike",
					Detail: fmt.Sprintf("%d calls with negative sentiment, floor avg %.2f", s.NegativeCalls, s.AverageSentiment),
				})
			}
		}
	}

	// ── Agent availability ────────────────────────────────────────────────────
	agentJSON, err := m.registry.Execute("get_agent_states", nil)
	if err == nil {
		var a struct {
			AvailableCount int `json:"available_count"`
			Total          int `json:"total"`
		}
		if json.Unmarshal([]byte(agentJSON), &a) == nil && a.Total > 0 {
			availPct := float64(a.AvailableCount) / float64(a.Total)
			if availPct <= 0.10 {
				out = append(out, anomaly{
					Level:  LevelCritical,
					Title:  "Critical agent shortage",
					Detail: fmt.Sprintf("only %d/%d agents available (%.0f%%)", a.AvailableCount, a.Total, availPct*100),
				})
			}
		}
	}

	return out
}

func (m *Monitor) generateAdvisory(anomalies []anomaly) {
	prompt := "Proactive monitor alert — current contact center anomalies detected:\n"
	for _, a := range anomalies {
		prompt += fmt.Sprintf("- [%s] %s: %s\n", a.Level, a.Title, a.Detail)
	}
	prompt += "\nProvide a brief (2-3 sentence) supervisor advisory with the single most important action to take right now."

	reply, err := m.loop.Chat(prompt)
	if err != nil {
		log.Printf("monitor: advisory generation failed: %v", err)
		return
	}

	m.hub.Broadcast(&models.Event{
		Type: models.EventTypeAlert,
		Payload: map[string]any{
			"source":  "ai_advisory",
			"level":   LevelWarning,
			"title":   "AI Advisory",
			"detail":  reply,
			"ts":      time.Now().Unix(),
		},
	})
	log.Printf("monitor: AI advisory broadcast")
}
