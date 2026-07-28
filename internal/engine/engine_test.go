package engine

import (
	"context"
	"testing"
	"time"

	"cortex-cc/internal/models"
	"cortex-cc/internal/store"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st, nil)
}

func injectCall(eng *Engine, id, queue string) {
	eng.mu.Lock()
	defer eng.mu.Unlock()
	eng.calls[id] = &models.Call{
		ID:        id,
		QueueName: queue,
		Status:    models.CallStatusQueued,
		StartedAt: time.Now(),
	}
}

func TestSeedAgents(t *testing.T) {
	eng := newTestEngine(t)
	eng.seedAgents()

	agents := eng.GetAgents()
	if len(agents) != 10 {
		t.Errorf("expected 10 seeded agents, got %d", len(agents))
	}
	for _, a := range agents {
		if a.Name == "" {
			t.Error("agent has empty name")
		}
		if len(a.Skills) == 0 {
			t.Errorf("agent %s has no skills", a.Name)
		}
	}
}

func TestGetActiveCallsEmpty(t *testing.T) {
	eng := newTestEngine(t)
	calls := eng.GetActiveCalls()
	if calls == nil {
		t.Error("expected non-nil slice, got nil")
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 calls on fresh engine, got %d", len(calls))
	}
}

func TestFlagCall(t *testing.T) {
	eng := newTestEngine(t)
	eng.seedAgents()
	injectCall(eng, "C-test01", "Billing")

	ok := eng.FlagCall("C-test01", "angry customer")
	if !ok {
		t.Fatal("FlagCall returned false for existing call")
	}

	eng.mu.RLock()
	c := eng.calls["C-test01"]
	eng.mu.RUnlock()

	if !c.Flagged {
		t.Error("expected call to be flagged")
	}
	if c.FlagReason != "angry customer" {
		t.Errorf("unexpected flag reason: %q", c.FlagReason)
	}
}

func TestFlagCallNotFound(t *testing.T) {
	eng := newTestEngine(t)
	if eng.FlagCall("C-nonexistent", "test") {
		t.Error("FlagCall should return false for unknown call ID")
	}
}

func TestRouteCallToAvailableAgent(t *testing.T) {
	eng := newTestEngine(t)
	eng.seedAgents()
	injectCall(eng, "C-route01", "Sales")

	agents := eng.GetAgents()
	if len(agents) == 0 {
		t.Skip("no agents seeded")
	}

	ok := eng.RouteCall("C-route01", agents[0].ID)
	if !ok {
		t.Fatal("RouteCall returned false for valid call + available agent")
	}

	eng.mu.RLock()
	c := eng.calls["C-route01"]
	eng.mu.RUnlock()

	if c.Status != models.CallStatusActive {
		t.Errorf("expected call status active after routing, got %s", c.Status)
	}
}

func TestRouteCallBusyAgent(t *testing.T) {
	eng := newTestEngine(t)
	eng.seedAgents()
	injectCall(eng, "C-route02", "Billing")

	// Mark all agents busy
	eng.mu.Lock()
	for _, a := range eng.agents {
		a.Status = models.AgentStatusBusy
	}
	eng.mu.Unlock()

	agents := eng.GetAgents()
	if len(agents) == 0 {
		t.Skip("no agents seeded")
	}

	ok := eng.RouteCall("C-route02", agents[0].ID)
	if ok {
		t.Error("RouteCall should fail when target agent is busy")
	}
}

func TestEngineStartAndStop(t *testing.T) {
	eng := newTestEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	eng.Start(ctx)
	<-ctx.Done()

	if len(eng.GetAgents()) != 10 {
		t.Errorf("expected 10 agents after start, got %d", len(eng.GetAgents()))
	}
}

func TestOnCustomerLineHook(t *testing.T) {
	eng := newTestEngine(t)
	called := make(chan string, 1)
	eng.OnCustomerLine = func(call *models.Call, text string) {
		called <- text
	}

	// Trigger the hook manually (simulates what transcript.go does)
	injectCall(eng, "C-hook01", "Support")
	eng.mu.RLock()
	c := eng.calls["C-hook01"]
	eng.mu.RUnlock()

	go eng.OnCustomerLine(c, "my internet is down")

	select {
	case text := <-called:
		if text != "my internet is down" {
			t.Errorf("unexpected hook text: %q", text)
		}
	case <-time.After(time.Second):
		t.Error("OnCustomerLine hook was never called")
	}
}
