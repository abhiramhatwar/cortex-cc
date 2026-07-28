package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"cortex-cc/internal/engine"
	"cortex-cc/internal/store"
)

func newTestRegistry(t *testing.T) *ToolRegistry {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	eng := engine.New(st, nil)
	eng.Start(t.Context())
	return NewToolRegistry(eng, st, nil)
}

func TestDefinitionsCount(t *testing.T) {
	r := newTestRegistry(t)
	defs := r.Definitions()
	if len(defs) != 9 {
		t.Errorf("expected 9 tool definitions, got %d", len(defs))
	}
}

func TestDefinitionNames(t *testing.T) {
	r := newTestRegistry(t)
	want := map[string]bool{
		"get_queue_status":    true,
		"get_agent_states":    true,
		"get_call_transcript": true,
		"get_sentiment_report": true,
		"route_call":          true,
		"flag_call":           true,
		"summarize_call":        true,
		"find_best_agent":       true,
		"search_knowledge_base": true,
	}
	for _, d := range r.Definitions() {
		name := d.Function.Name
		if !want[name] {
			t.Errorf("unexpected tool name: %q", name)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("missing expected tool: %q", name)
	}
}

func TestDefinitionsHaveDescriptions(t *testing.T) {
	r := newTestRegistry(t)
	for _, d := range r.Definitions() {
		if strings.TrimSpace(d.Function.Description) == "" {
			t.Errorf("tool %q has empty description", d.Function.Name)
		}
		if d.Type != "function" {
			t.Errorf("tool %q has unexpected type %q", d.Function.Name, d.Type)
		}
	}
}

func TestDefinitionsParametersAreValidJSON(t *testing.T) {
	r := newTestRegistry(t)
	for _, d := range r.Definitions() {
		var v any
		if err := json.Unmarshal(d.Function.Parameters, &v); err != nil {
			t.Errorf("tool %q parameters are not valid JSON: %v", d.Function.Name, err)
		}
	}
}

func TestExecuteGetQueueStatus(t *testing.T) {
	r := newTestRegistry(t)
	result, err := r.Execute("get_queue_status", nil)
	if err != nil {
		t.Fatalf("get_queue_status: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(result), &v); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if _, ok := v["queues"]; !ok {
		t.Error("result missing 'queues' key")
	}
}

func TestExecuteGetAgentStates(t *testing.T) {
	r := newTestRegistry(t)
	result, err := r.Execute("get_agent_states", nil)
	if err != nil {
		t.Fatalf("get_agent_states: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(result), &v); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if _, ok := v["agents"]; !ok {
		t.Error("result missing 'agents' key")
	}
}

func TestExecuteGetSentimentReport(t *testing.T) {
	r := newTestRegistry(t)
	result, err := r.Execute("get_sentiment_report", nil)
	if err != nil {
		t.Fatalf("get_sentiment_report: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(result), &v); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
}

func TestExecuteUnknownTool(t *testing.T) {
	r := newTestRegistry(t)
	_, err := r.Execute("does_not_exist", nil)
	if err == nil {
		t.Error("expected error for unknown tool, got nil")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExecuteFlagCallMissingID(t *testing.T) {
	r := newTestRegistry(t)
	_, err := r.Execute("flag_call", map[string]any{})
	if err == nil {
		t.Error("expected error when call_id is missing")
	}
}

func TestExecuteGetTranscriptMissingID(t *testing.T) {
	r := newTestRegistry(t)
	_, err := r.Execute("get_call_transcript", map[string]any{})
	if err == nil {
		t.Error("expected error when call_id is missing")
	}
}
