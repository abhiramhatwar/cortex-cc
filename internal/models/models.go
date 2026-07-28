package models

import "time"

type CallStatus string
type AgentStatus string

const (
	CallStatusQueued     CallStatus = "queued"
	CallStatusRinging    CallStatus = "ringing"
	CallStatusActive     CallStatus = "active"
	CallStatusOnHold     CallStatus = "on_hold"
	CallStatusCompleted  CallStatus = "completed"
	CallStatusAbandoned  CallStatus = "abandoned"

	AgentStatusAvailable AgentStatus = "available"
	AgentStatusBusy      AgentStatus = "busy"
	AgentStatusOnBreak   AgentStatus = "on_break"
	AgentStatusOffline   AgentStatus = "offline"
)

type Agent struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Status       AgentStatus `json:"status"`
	CurrentCallID string     `json:"current_call_id,omitempty"`
	CallsHandled int         `json:"calls_handled"`
	AvgHandleTime float64    `json:"avg_handle_time_seconds"`
	Skills        []string   `json:"skills"`
}

type Call struct {
	ID           string     `json:"id"`
	CallerNumber string     `json:"caller_number"`
	CallerName   string     `json:"caller_name,omitempty"`
	AgentID      string     `json:"agent_id,omitempty"`
	QueueName    string     `json:"queue_name"`
	Status       CallStatus `json:"status"`
	WaitSeconds  int        `json:"wait_seconds"`
	TalkSeconds  int        `json:"talk_seconds"`
	Sentiment    float64    `json:"sentiment"` // -1.0 (negative) to 1.0 (positive)
	SLABreached  bool       `json:"sla_breached"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	Flagged      bool       `json:"flagged"`
	FlagReason   string     `json:"flag_reason,omitempty"`
	RoutingNote  string     `json:"routing_note,omitempty"`
}

type Transcript struct {
	ID        string    `json:"id"`
	CallID    string    `json:"call_id"`
	Speaker   string    `json:"speaker"` // "agent" | "customer"
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

type CallSummary struct {
	CallID     string    `json:"call_id"`
	Issue      string    `json:"issue"`
	Resolution string    `json:"resolution"`
	FollowUp   string    `json:"follow_up"`
	Sentiment  string    `json:"sentiment_label"` // positive | neutral | negative
	CreatedAt  time.Time `json:"created_at"`
}

type QueueStats struct {
	QueueName      string  `json:"queue_name"`
	ActiveCalls    int     `json:"active_calls"`
	WaitingCalls   int     `json:"waiting_calls"`
	AvailableAgents int    `json:"available_agents"`
	AvgWaitSeconds float64 `json:"avg_wait_seconds"`
	SLABreachCount int     `json:"sla_breach_count"`
	AbandonRate    float64 `json:"abandon_rate"`
}

const (
	EventTypeCallUpdate  = "call_update"
	EventTypeAgentUpdate = "agent_update"
	EventTypeAlert       = "alert"
)

type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type KBArticle struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
}

type CallScore struct {
	CallID          string    `json:"call_id"`
	Empathy         int       `json:"empathy"`         // 1-10
	Resolution      int       `json:"resolution"`      // 1-10
	Professionalism int       `json:"professionalism"` // 1-10
	Overall         int       `json:"overall"`         // 1-10
	Notes           string    `json:"notes"`
	ScoredAt        time.Time `json:"scored_at"`
}
