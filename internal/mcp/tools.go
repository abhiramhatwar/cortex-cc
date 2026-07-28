package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"cortex-cc/internal/engine"
	"cortex-cc/internal/models"
	"cortex-cc/internal/store"
)

// ToolRegistry holds all MCP tool definitions and their handlers.
// It is used both by the MCP server (for external MCP clients)
// and by the Ollama loop (for LLM tool calling).
type ToolRegistry struct {
	engine *engine.Engine
	store  *store.Store
}

func NewToolRegistry(eng *engine.Engine, st *store.Store) *ToolRegistry {
	return &ToolRegistry{engine: eng, store: st}
}

// ToolDef is what we send to Ollama as a function definition.
type ToolDef struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Definitions returns all tool definitions for Ollama.
func (r *ToolRegistry) Definitions() []ToolDef { return allDefinitions() }

// allDefinitions is the single source of truth for all tool metadata.
// Both ToolRegistry and HTTPExecutor call this.
func allDefinitions() []ToolDef {
	return []ToolDef{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_queue_status",
				Description: "Returns the live status of all call queues: active calls, calls waiting, average wait time, SLA breaches, and abandon rate. Use this to understand queue health.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_agent_states",
				Description: "Returns all agents with their current status (available, busy, on_break), which call they are handling, total calls handled today, and average handle time in seconds.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_call_transcript",
				Description: "Returns the full conversation transcript for a given call ID. Use this to understand what was said on a specific call.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"call_id":{"type":"string","description":"The call ID, e.g. C-a1b2c3"}
					},
					"required":["call_id"]
				}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_sentiment_report",
				Description: "Returns sentiment scores for all active calls. Sentiment is a float from -1.0 (very negative) to 1.0 (very positive). Use this to spot distressed customers.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "route_call",
				Description: "Transfers a call to a specific agent. The agent must be available. Use this when the supervisor asks to move a call.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"call_id":{"type":"string","description":"The call ID to transfer"},
						"agent_id":{"type":"string","description":"The agent ID to transfer to"}
					},
					"required":["call_id","agent_id"]
				}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "flag_call",
				Description: "Flags a call for QA review with a reason. Use when a supervisor asks to flag or escalate a call.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"call_id":{"type":"string","description":"The call ID to flag"},
						"reason":{"type":"string","description":"Reason for flagging, e.g. 'angry customer', 'billing dispute'"}
					},
					"required":["call_id","reason"]
				}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "summarize_call",
				Description: "Generates a structured post-call summary for a completed call: issue, resolution, follow-up action, and sentiment label.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"call_id":{"type":"string","description":"The call ID to summarize"}
					},
					"required":["call_id"]
				}`),
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "find_best_agent",
				Description: "Returns the best available agent for a given queue using skills-based scoring: primary skill match > secondary skill > fallback, then tiebroken by fewest calls handled today. Also returns a human-readable routing reason.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"queue":{"type":"string","description":"Queue name: Sales, Billing, or Support"}
					},
					"required":["queue"]
				}`),
			},
		},
	}
}

// Execute dispatches a tool call by name and returns a JSON string result.
func (r *ToolRegistry) Execute(name string, args map[string]any) (string, error) {
	switch name {
	case "get_queue_status":
		return r.getQueueStatus()
	case "get_agent_states":
		return r.getAgentStates()
	case "get_call_transcript":
		id, _ := args["call_id"].(string)
		return r.getCallTranscript(id)
	case "get_sentiment_report":
		return r.getSentimentReport()
	case "route_call":
		callID, _ := args["call_id"].(string)
		agentID, _ := args["agent_id"].(string)
		return r.routeCall(callID, agentID)
	case "flag_call":
		callID, _ := args["call_id"].(string)
		reason, _ := args["reason"].(string)
		return r.flagCall(callID, reason)
	case "summarize_call":
		id, _ := args["call_id"].(string)
		return r.summarizeCall(id)
	case "find_best_agent":
		queue, _ := args["queue"].(string)
		return r.findBestAgent(queue)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// ── Tool implementations ──────────────────────────────────────────────────────

func (r *ToolRegistry) getQueueStatus() (string, error) {
	calls := r.engine.GetActiveCalls()

	type queueData struct {
		Name           string  `json:"queue_name"`
		ActiveCalls    int     `json:"active_calls"`
		WaitingCalls   int     `json:"waiting_calls"`
		AvgWaitSeconds float64 `json:"avg_wait_seconds"`
		SLABreaches    int     `json:"sla_breaches"`
		LongestWait    int     `json:"longest_wait_seconds"`
	}
	qmap := map[string]*queueData{}
	for _, c := range calls {
		q := qmap[c.QueueName]
		if q == nil {
			q = &queueData{Name: c.QueueName}
			qmap[c.QueueName] = q
		}
		switch c.Status {
		case models.CallStatusActive:
			q.ActiveCalls++
		case models.CallStatusQueued:
			q.WaitingCalls++
			q.AvgWaitSeconds += float64(c.WaitSeconds)
			if c.WaitSeconds > q.LongestWait {
				q.LongestWait = c.WaitSeconds
			}
		}
		if c.SLABreached {
			q.SLABreaches++
		}
	}
	for _, q := range qmap {
		if q.WaitingCalls > 0 {
			q.AvgWaitSeconds /= float64(q.WaitingCalls)
		}
	}
	out := make([]*queueData, 0, len(qmap))
	for _, q := range qmap {
		out = append(out, q)
	}
	return marshal(map[string]any{"queues": out, "total_active_calls": len(calls)})
}

func (r *ToolRegistry) getAgentStates() (string, error) {
	agents := r.engine.GetAgents()
	sort.Slice(agents, func(i, j int) bool { return agents[i].Name < agents[j].Name })
	type agentView struct {
		ID            string  `json:"id"`
		Name          string  `json:"name"`
		Status        string  `json:"status"`
		CurrentCallID string  `json:"current_call_id,omitempty"`
		CallsHandled  int     `json:"calls_handled_today"`
		AvgHandleTime float64 `json:"avg_handle_time_seconds"`
		Skills        []string `json:"skills"`
	}
	views := make([]agentView, len(agents))
	for i, a := range agents {
		views[i] = agentView{
			ID: a.ID, Name: a.Name, Status: string(a.Status),
			CurrentCallID: a.CurrentCallID, CallsHandled: a.CallsHandled,
			AvgHandleTime: a.AvgHandleTime, Skills: a.Skills,
		}
	}
	available := 0
	for _, a := range agents {
		if a.Status == models.AgentStatusAvailable {
			available++
		}
	}
	return marshal(map[string]any{"agents": views, "available_count": available, "total": len(agents)})
}

func (r *ToolRegistry) getCallTranscript(callID string) (string, error) {
	if callID == "" {
		return "", fmt.Errorf("call_id is required")
	}
	lines, err := r.store.GetTranscriptByCallID(callID)
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return marshal(map[string]any{"call_id": callID, "transcript": []any{}, "message": "no transcript yet"})
	}
	type line struct {
		Speaker   string `json:"speaker"`
		Text      string `json:"text"`
		Timestamp string `json:"timestamp"`
	}
	out := make([]line, len(lines))
	for i, l := range lines {
		out[i] = line{Speaker: l.Speaker, Text: l.Text, Timestamp: l.Timestamp.Format(time.RFC3339)}
	}
	return marshal(map[string]any{"call_id": callID, "transcript": out, "line_count": len(out)})
}

func (r *ToolRegistry) getSentimentReport() (string, error) {
	calls := r.engine.GetActiveCalls()
	type sentView struct {
		CallID     string  `json:"call_id"`
		CallerName string  `json:"caller_name"`
		Queue      string  `json:"queue"`
		Sentiment  float64 `json:"sentiment"`
		Label      string  `json:"label"`
	}
	views := make([]sentView, 0, len(calls))
	for _, c := range calls {
		if c.Status != models.CallStatusActive {
			continue
		}
		label := "neutral"
		if c.Sentiment > 0.3 {
			label = "positive"
		} else if c.Sentiment < -0.3 {
			label = "negative"
		}
		views = append(views, sentView{
			CallID: c.ID, CallerName: c.CallerName,
			Queue: c.QueueName, Sentiment: c.Sentiment, Label: label,
		})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Sentiment < views[j].Sentiment })

	avgSent := 0.0
	for _, v := range views {
		avgSent += v.Sentiment
	}
	if len(views) > 0 {
		avgSent /= float64(len(views))
	}
	return marshal(map[string]any{
		"active_calls":       views,
		"average_sentiment":  avgSent,
		"negative_calls":     countWhere(views, func(v sentView) bool { return v.Label == "negative" }),
	})
}

func (r *ToolRegistry) routeCall(callID, agentID string) (string, error) {
	if callID == "" || agentID == "" {
		return "", fmt.Errorf("call_id and agent_id are required")
	}
	ok := r.engine.RouteCall(callID, agentID)
	if !ok {
		return marshal(map[string]any{
			"success": false,
			"message": fmt.Sprintf("could not route call %s to agent %s — agent may be unavailable or call not found", callID, agentID),
		})
	}
	return marshal(map[string]any{
		"success": true,
		"message": fmt.Sprintf("call %s has been transferred to agent %s", callID, agentID),
	})
}

func (r *ToolRegistry) flagCall(callID, reason string) (string, error) {
	if callID == "" {
		return "", fmt.Errorf("call_id is required")
	}
	ok := r.engine.FlagCall(callID, reason)
	if !ok {
		return marshal(map[string]any{"success": false, "message": "call not found: " + callID})
	}
	return marshal(map[string]any{
		"success": true,
		"message": fmt.Sprintf("call %s flagged for QA review: %s", callID, reason),
	})
}

func (r *ToolRegistry) summarizeCall(callID string) (string, error) {
	if callID == "" {
		return "", fmt.Errorf("call_id is required")
	}
	lines, err := r.store.GetTranscriptByCallID(callID)
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return marshal(map[string]any{"message": "no transcript available to summarize"})
	}
	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", l.Speaker, l.Text))
	}
	return marshal(map[string]any{
		"call_id":    callID,
		"transcript": sb.String(),
		"instruction": "Generate a JSON summary with fields: issue, resolution, follow_up, sentiment_label (positive/neutral/negative)",
	})
}

func (r *ToolRegistry) findBestAgent(queue string) (string, error) {
	if queue == "" {
		return "", fmt.Errorf("queue is required")
	}
	agent, reason := r.engine.BestAgentFor(queue)
	if agent == nil {
		return marshal(map[string]any{
			"queue":  queue,
			"agent":  nil,
			"reason": reason,
		})
	}
	return marshal(map[string]any{
		"queue":  queue,
		"reason": reason,
		"agent": map[string]any{
			"id":                    agent.ID,
			"name":                  agent.Name,
			"status":                agent.Status,
			"skills":                agent.Skills,
			"calls_handled_today":   agent.CallsHandled,
			"avg_handle_time_seconds": agent.AvgHandleTime,
		},
	})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func marshal(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	return string(b), err
}

type sentView struct {
	Label string
}

func countWhere[T any](s []T, f func(T) bool) int {
	n := 0
	for _, v := range s {
		if f(v) {
			n++
		}
	}
	return n
}
