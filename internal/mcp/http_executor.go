package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"cortex-cc/internal/models"
)

// HTTPExecutor implements Executor by calling the cortex-cc REST API.
// Used by the standalone MCP stdio server binary so it can access live
// engine data without sharing the same process.
type HTTPExecutor struct {
	baseURL string
	client  *http.Client
}

func NewHTTPExecutor(baseURL string) *HTTPExecutor {
	return &HTTPExecutor{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (h *HTTPExecutor) Definitions() []ToolDef { return allDefinitions() }

func (h *HTTPExecutor) Execute(name string, args map[string]any) (string, error) {
	switch name {
	case "get_queue_status":
		return h.getQueueStatus()
	case "get_agent_states":
		return h.getAgentStates()
	case "get_call_transcript":
		id, _ := args["call_id"].(string)
		return h.getCallTranscript(id)
	case "get_sentiment_report":
		return h.getSentimentReport()
	case "route_call":
		callID, _ := args["call_id"].(string)
		agentID, _ := args["agent_id"].(string)
		return h.routeCall(callID, agentID)
	case "flag_call":
		callID, _ := args["call_id"].(string)
		reason, _ := args["reason"].(string)
		return h.flagCall(callID, reason)
	case "summarize_call":
		id, _ := args["call_id"].(string)
		return h.summarizeCall(id)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (h *HTTPExecutor) get(path string, out any) error {
	resp, err := h.client.Get(h.baseURL + path)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GET %s returned %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (h *HTTPExecutor) postJSON(path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := h.client.Post(h.baseURL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

// ── tool implementations ──────────────────────────────────────────────────────

func (h *HTTPExecutor) getQueueStatus() (string, error) {
	var calls []*models.Call
	if err := h.get("/api/calls", &calls); err != nil {
		return "", err
	}
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

func (h *HTTPExecutor) getAgentStates() (string, error) {
	var agents []*models.Agent
	if err := h.get("/api/agents", &agents); err != nil {
		return "", err
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Name < agents[j].Name })
	type agentView struct {
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		Status        string   `json:"status"`
		CurrentCallID string   `json:"current_call_id,omitempty"`
		CallsHandled  int      `json:"calls_handled_today"`
		AvgHandleTime float64  `json:"avg_handle_time_seconds"`
		Skills        []string `json:"skills"`
	}
	views := make([]agentView, len(agents))
	available := 0
	for i, a := range agents {
		views[i] = agentView{
			ID: a.ID, Name: a.Name, Status: string(a.Status),
			CurrentCallID: a.CurrentCallID, CallsHandled: a.CallsHandled,
			AvgHandleTime: a.AvgHandleTime, Skills: a.Skills,
		}
		if a.Status == models.AgentStatusAvailable {
			available++
		}
	}
	return marshal(map[string]any{"agents": views, "available_count": available, "total": len(agents)})
}

func (h *HTTPExecutor) getSentimentReport() (string, error) {
	var calls []*models.Call
	if err := h.get("/api/calls", &calls); err != nil {
		return "", err
	}
	type sentView struct {
		CallID     string  `json:"call_id"`
		CallerName string  `json:"caller_name"`
		Queue      string  `json:"queue"`
		Sentiment  float64 `json:"sentiment"`
		Label      string  `json:"label"`
	}
	views := make([]sentView, 0)
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
	avg := 0.0
	neg := 0
	for _, v := range views {
		avg += v.Sentiment
		if v.Label == "negative" {
			neg++
		}
	}
	if len(views) > 0 {
		avg /= float64(len(views))
	}
	return marshal(map[string]any{"active_calls": views, "average_sentiment": avg, "negative_calls": neg})
}

func (h *HTTPExecutor) getCallTranscript(callID string) (string, error) {
	if callID == "" {
		return "", fmt.Errorf("call_id is required")
	}
	var lines []*models.Transcript
	if err := h.get("/api/calls/"+callID+"/transcript", &lines); err != nil {
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

func (h *HTTPExecutor) flagCall(callID, reason string) (string, error) {
	if callID == "" {
		return "", fmt.Errorf("call_id is required")
	}
	var result map[string]any
	if err := h.postJSON("/api/calls/"+callID+"/flag", map[string]string{"reason": reason}, &result); err != nil {
		return "", err
	}
	return marshal(result)
}

func (h *HTTPExecutor) routeCall(callID, agentID string) (string, error) {
	if callID == "" || agentID == "" {
		return "", fmt.Errorf("call_id and agent_id are required")
	}
	var result map[string]any
	if err := h.postJSON("/api/calls/"+callID+"/route", map[string]string{"agent_id": agentID}, &result); err != nil {
		return "", err
	}
	return marshal(result)
}

func (h *HTTPExecutor) summarizeCall(callID string) (string, error) {
	if callID == "" {
		return "", fmt.Errorf("call_id is required")
	}
	var lines []*models.Transcript
	if err := h.get("/api/calls/"+callID+"/transcript", &lines); err != nil {
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
		"call_id":     callID,
		"transcript":  sb.String(),
		"instruction": "Generate a JSON summary with fields: issue, resolution, follow_up, sentiment_label (positive/neutral/negative)",
	})
}
