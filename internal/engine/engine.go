package engine

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/abhiram/cortex-cc/internal/models"
	"github.com/abhiram/cortex-cc/internal/store"
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
	store  *store.Store
	Events chan *models.Event

	mu     sync.RWMutex
	agents map[string]*models.Agent
	calls  map[string]*models.Call
}

func New(st *store.Store) *Engine {
	return &Engine{
		store:  st,
		Events: make(chan *models.Event, 256),
		agents: make(map[string]*models.Agent),
		calls:  make(map[string]*models.Call),
	}
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
