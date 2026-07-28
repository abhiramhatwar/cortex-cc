package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

func NewClient(baseURL, model string) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		http:    &http.Client{Timeout: 90 * time.Second},
	}
}

// ── Ollama API types ──────────────────────────────────────────────────────────

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type chatRequest struct {
	Model    string      `json:"model"`
	Messages []Message   `json:"messages"`
	Tools    any         `json:"tools,omitempty"`
	Stream   bool        `json:"stream"`
}

type chatResponse struct {
	Message chatResponseMessage `json:"message"`
	Done    bool                `json:"done"`
}

type chatResponseMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Chat sends messages to Ollama and returns the assistant response.
func (c *Client) Chat(messages []Message, tools any) (*chatResponseMessage, error) {
	body, err := json.Marshal(chatRequest{
		Model:    c.model,
		Messages: messages,
		Tools:    tools,
		Stream:   false,
	})
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Post(c.baseURL+"/api/chat", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama unreachable: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, raw)
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &cr.Message, nil
}

// OneShot sends a single system+user turn with no tools and returns the raw text reply.
// Used by the QA scorer to generate structured scores without multi-turn history.
func (c *Client) OneShot(system, prompt string) (string, error) {
	msg, err := c.Chat([]Message{
		{Role: "system", Content: system},
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		return "", err
	}
	return msg.Content, nil
}

// Ping checks if Ollama is reachable and the model is available.
func (c *Client) Ping() error {
	resp, err := c.http.Get(c.baseURL + "/api/tags")
	if err != nil {
		return fmt.Errorf("ollama not reachable at %s: %w", c.baseURL, err)
	}
	resp.Body.Close()
	return nil
}
