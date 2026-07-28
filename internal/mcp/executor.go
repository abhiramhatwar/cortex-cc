package mcp

// Executor is the common interface for tool dispatching.
// ToolRegistry implements it using direct engine access (in-process).
// HTTPExecutor implements it by calling the cortex-cc REST API (out-of-process).
type Executor interface {
	Definitions() []ToolDef
	Execute(name string, args map[string]any) (string, error)
}
