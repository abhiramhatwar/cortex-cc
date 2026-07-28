package llm

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"cortex-cc/internal/mcp"
)

const systemPrompt = `You are cortex-cc, an AI copilot for a contact center supervisor.
You have access to live contact center data via tools. Always call the relevant tools
before answering — never guess at call IDs, agent names, or queue stats.

Guidelines:
- Be concise and actionable. Supervisors are busy.
- When asked about struggling agents, call get_agent_states AND get_sentiment_report.
- When routing or flagging, confirm the action was successful.
- If Ollama cannot reach a tool, say so honestly.
- Format numbers clearly: "4 min 30 sec" not "270 seconds".`

const maxToolRounds = 5 // prevent infinite loops

// Loop manages conversation history and the agentic tool-calling cycle.
type Loop struct {
	client   *Client
	registry *mcp.ToolRegistry
	mu       sync.Mutex
	history  []Message // shared across turns for multi-turn chat
}

func NewLoop(client *Client, registry *mcp.ToolRegistry) *Loop {
	return &Loop{
		client:   client,
		registry: registry,
		history:  []Message{{Role: "system", Content: systemPrompt}},
	}
}

const maxHistoryMessages = 20 // system prompt + this many messages before trimming

// Chat sends a user message, runs the tool loop, and returns the final reply.
func (l *Loop) Chat(userMsg string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Keep history bounded so we never exceed the model's context window.
	if len(l.history) > maxHistoryMessages {
		l.history = append(l.history[:1], l.history[len(l.history)-maxHistoryMessages+1:]...)
	}
	l.history = append(l.history, Message{Role: "user", Content: userMsg})
	tools := l.registry.Definitions()

	for round := 0; round < maxToolRounds; round++ {
		resp, err := l.client.Chat(l.history, tools)
		if err != nil {
			return "", fmt.Errorf("llm: %w", err)
		}

		// No tool calls → final text answer
		if len(resp.ToolCalls) == 0 {
			reply := strings.TrimSpace(resp.Content)
			l.history = append(l.history, Message{Role: "assistant", Content: reply})
			return reply, nil
		}

		// Execute each tool call and collect results
		l.history = append(l.history, Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			name := tc.Function.Name
			args := tc.Function.Arguments
			log.Printf("llm: calling tool %s with args %v", name, args)

			result, err := l.registry.Execute(name, args)
			if err != nil {
				result = fmt.Sprintf(`{"error": "%s"}`, err.Error())
				log.Printf("llm: tool %s error: %v", name, err)
			}

			l.history = append(l.history, Message{
				Role:    "tool",
				Content: result,
			})
		}
		// loop again — Ollama will reason over tool results
	}

	return "I was unable to complete the request after multiple tool calls. Please try again.", nil
}

// ResetHistory clears conversation context (e.g. new supervisor session).
func (l *Loop) ResetHistory() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.history = []Message{{Role: "system", Content: systemPrompt}}
}

// OneShot generates a single AI response without touching the shared conversation history.
// Used by background services (monitor, etc.) that must not inject prompts into the supervisor chat.
func (l *Loop) OneShot(system, prompt string) (string, error) {
	return l.client.OneShot(system, prompt)
}
