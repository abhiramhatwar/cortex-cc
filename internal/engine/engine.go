package engine

import (
	"context"
	"fmt"
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

	// OnCallCompleted is called asynchronously when a call transitions to completed.
	// Set this to wire in the QA scorer without a hard import dependency.
	OnCallCompleted func(call *models.Call)

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
	out := make([]*models.Call, 0)
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
			// assign to best-scored available agent
			if r := e.findBestAgentFor(c.QueueName); r != nil {
				c.Status = models.CallStatusActive
				c.AgentID = r.agent.ID
				c.RoutingNote = r.reason
				r.agent.Status = models.AgentStatusBusy
				r.agent.CurrentCallID = c.ID
				e.store.UpsertAgent(r.agent)
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
	if e.OnCallCompleted != nil {
		cp := *c
		go e.OnCallCompleted(&cp)
	}
}

// BestAgentFor returns the highest-scoring available agent for a queue and
// a human-readable explanation of why they were selected. Safe to call from
// outside the engine — acquires its own read lock.
func (e *Engine) BestAgentFor(queue string) (*models.Agent, string) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	r := e.findBestAgentFor(queue)
	if r == nil {
		return nil, "no available agents for " + queue
	}
	return r.agent, r.reason
}

type routeResult struct {
	agent  *models.Agent
	score  int
	reason string
}

// findBestAgentFor selects the best available agent using a three-tier score:
//
//	2 = primary skill (queue is agent's first/preferred skill)
//	1 = secondary skill (queue is in agent's skill list but not primary)
//	0 = no-skill fallback (any available agent)
//
// Tiebreaks: fewest calls handled today, then lower average handle time.
// Must be called with e.mu held (read or write).
func (e *Engine) findBestAgentFor(queue string) *routeResult {
	var best *routeResult
	for _, a := range e.agents {
		if a.Status != models.AgentStatusAvailable {
			continue
		}
		score := 0
		if len(a.Skills) > 0 && a.Skills[0] == queue {
			score = 2
		} else if hasSkill(a.Skills, queue) {
			score = 1
		}
		if best == nil ||
			score > best.score ||
			(score == best.score && a.CallsHandled < best.agent.CallsHandled) ||
			(score == best.score && a.CallsHandled == best.agent.CallsHandled && a.AvgHandleTime < best.agent.AvgHandleTime) {
			r := &routeResult{agent: a, score: score}
			switch score {
			case 2:
				r.reason = fmt.Sprintf("primary skill: %s specialist (%d calls today)", queue, a.CallsHandled)
			case 1:
				r.reason = fmt.Sprintf("secondary skill: multi-skilled agent (%d calls today)", a.CallsHandled)
			default:
				r.reason = fmt.Sprintf("no-skill fallback: least-loaded available agent (%d calls today)", a.CallsHandled)
			}
			best = r
		}
	}
	return best
}
