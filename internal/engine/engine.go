package engine

import (
	"context"
	"log"
	"sync"
	"time"

	"cortex-cc/internal/models"
	"cortex-cc/internal/sentiment"
	"cortex-cc/internal/store"
	"github.com/google/uuid"
)

const (
	slaThresholdSeconds = 120
	tickInterval        = 2 * time.Second
	callGenInterval     = 8 * time.Second
	transcriptInterval  = 6 * time.Second
)

var queues = []string{"Sales", "Billing", "Support"}

type Engine struct {
	store     *store.Store
	sentiment *sentiment.Client // optional — nil if service not running
	Events    chan *models.Event

	// OnCustomerLine is called asynchronously for every customer transcript line.
	// Set this to wire in the agent assist service without a hard import dependency.
	OnCustomerLine func(call *models.Call, text string)

	mu     sync.RWMutex
	agents map[string]*models.Agent
	calls  map[string]*models.Call
}

func New(st *store.Store, sc *sentiment.Client) *Engine {
	return &Engine{
		store:     st,
		sentiment: sc,
		Events:    make(chan *models.Event, 256),
		agents:    make(map[string]*models.Agent),
		calls:     make(map[string]*models.Call),
	}
}

// UpdateSentiment scores a transcript line and updates the call's running sentiment.
// Called asynchronously from the transcript generator. No-op if sentiment is offline.
func (e *Engine) UpdateSentiment(callID, text string) {
	if e.sentiment == nil {
		return
	}
	score, err := e.sentiment.Score(text)
	if err != nil {
		log.Printf("sentiment: %v", err)
		return
	}
	e.mu.Lock()
	if c, ok := e.calls[callID]; ok {
		// Exponential moving average — recent lines weigh more
		c.Sentiment = 0.7*c.Sentiment + 0.3*score
		e.emit(models.EventTypeAlert, map[string]any{
			"source": "sentiment",
			"call_id": callID,
			"score":  c.Sentiment,
		})
	}
	e.mu.Unlock()
}

func (e *Engine) Start(ctx context.Context) {
	e.seedAgents()
	go e.runTicker(ctx)
	go e.runCallGenerator(ctx)
	go e.runTranscriptGenerator(ctx)
	log.Println("engine: started")
}

// ── Snapshot accessors (used by MCP tools) ───────────────────────────────────

func (e *Engine) GetAgents() []*models.Agent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*models.Agent, 0, len(e.agents))
	for _, a := range e.agents {
		cp := *a
		out = append(out, &cp)
	}
	return out
}

func (e *Engine) GetActiveCalls() []*models.Call {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var out []*models.Call
	for _, c := range e.calls {
		cp := *c
		out = append(out, &cp)
	}
	return out
}

func (e *Engine) RouteCall(callID, agentID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	c, ok := e.calls[callID]
	if !ok {
		return false
	}
	a, ok := e.agents[agentID]
	if !ok || a.Status != models.AgentStatusAvailable {
		return false
	}
	c.AgentID = agentID
	c.Status = models.CallStatusActive
	a.Status = models.AgentStatusBusy
	a.CurrentCallID = callID
	e.emit("call_routed", c)
	return true
}

func (e *Engine) FlagCall(callID, reason string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	c, ok := e.calls[callID]
	if !ok {
		return false
	}
	c.Flagged = true
	c.FlagReason = reason
	e.store.UpsertCall(c)
	e.emit("call_flagged", c)
	return true
}

// ── Agent seeding ─────────────────────────────────────────────────────────────

func (e *Engine) seedAgents() {
	seed := []struct {
		name   string
		skills []string
	}{
		{"Sarah Mitchell", []string{"Sales", "Billing"}},
		{"James Okafor", []string{"Support", "Sales"}},
		{"Priya Sharma", []string{"Billing", "Support"}},
		{"Tom Heffner", []string{"Sales"}},
		{"Aiko Tanaka", []string{"Support", "Billing"}},
		{"Carlos Rivera", []string{"Billing"}},
		{"Emma Lawson", []string{"Sales", "Support"}},
		{"Raj Patel", []string{"Support"}},
		{"Nina Volkov", []string{"Billing", "Sales"}},
		{"Leo Martins", []string{"Sales", "Support", "Billing"}},
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for _, s := range seed {
		a := &models.Agent{
			ID:     uuid.NewString(),
			Name:   s.name,
			Status: models.AgentStatusAvailable,
			Skills: s.skills,
		}
		e.agents[a.ID] = a
		e.store.UpsertAgent(a)
	}
	log.Printf("engine: seeded %d agents", len(seed))
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (e *Engine) emit(eventType string, payload any) {
	select {
	case e.Events <- &models.Event{Type: eventType, Payload: payload}:
	default:
	}
}

func (e *Engine) newCallID() string {
	return "C-" + uuid.NewString()[:6]
}

// ── Ticker — advances call timers and SLA ─────────────────────────────────────

func (e *Engine) runTicker(ctx context.Context) {
	t := time.NewTicker(tickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.tick()
		}
	}
}

func (e *Engine) tick() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	for _, c := range e.calls {
		elapsed := int(now.Sub(c.StartedAt).Seconds())

		switch c.Status {
		case models.CallStatusQueued:
			c.WaitSeconds = elapsed
			if elapsed > slaThresholdSeconds && !c.SLABreached {
				c.SLABreached = true
				e.emit("sla_breached", c)
			}
			// 3% chance per tick to abandon
			if elapsed > 30 && randN(100) < 3 {
				c.Status = models.CallStatusAbandoned
				t := now
				c.EndedAt = &t
				delete(e.calls, c.ID)
				e.store.UpsertCall(c)
				e.emit("call_abandoned", c)
				continue
			}
			// assign to available agent
			if agent := e.findAvailableAgentFor(c.QueueName); agent != nil {
				c.Status = models.CallStatusActive
				c.AgentID = agent.ID
				agent.Status = models.AgentStatusBusy
				agent.CurrentCallID = c.ID
				e.store.UpsertAgent(agent)
				e.emit("call_answered", c)
			}

		case models.CallStatusActive:
			c.TalkSeconds = elapsed - c.WaitSeconds
			// drift sentiment slightly
			c.Sentiment = clamp(c.Sentiment+randFloat(-0.05, 0.05), -1, 1)
			if c.TalkSeconds > 0 && c.TalkSeconds%60 == 0 {
				e.emit("sentiment_update", c)
			}
			// complete after 60-300s of talk
			if c.TalkSeconds > 60 && randN(100) < 4 {
				e.completeCall(c, now)
			}

		case models.CallStatusOnHold:
			c.WaitSeconds += int(tickInterval.Seconds())
		}

		e.store.UpsertCall(c)
		e.emit("call_updated", c)
	}
}

func (e *Engine) completeCall(c *models.Call, now time.Time) {
	c.Status = models.CallStatusCompleted
	c.EndedAt = &now
	if agent, ok := e.agents[c.AgentID]; ok {
		agent.Status = models.AgentStatusAvailable
		agent.CurrentCallID = ""
		agent.CallsHandled++
		agent.AvgHandleTime = (agent.AvgHandleTime*float64(agent.CallsHandled-1) +
			float64(c.TalkSeconds)) / float64(agent.CallsHandled)
		e.store.UpsertAgent(agent)
	}
	delete(e.calls, c.ID)
	e.store.UpsertCall(c)
	e.emit("call_completed", c)
}

func (e *Engine) findAvailableAgentFor(queue string) *models.Agent {
	for _, a := range e.agents {
		if a.Status == models.AgentStatusAvailable && hasSkill(a.Skills, queue) {
			return a
		}
	}
	return nil
}
