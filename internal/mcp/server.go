package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MCPServer wraps the MCP SDK server and exposes the tool registry
// over the Model Context Protocol (stdio transport).
// This lets any MCP-compatible client (Claude Desktop, Cursor, etc.)
// connect directly to cortex-cc and call contact center tools.
type MCPServer struct {
	registry *ToolRegistry
	srv      *server.MCPServer
}

func NewMCPServer(registry *ToolRegistry) *MCPServer {
	s := &MCPServer{registry: registry}
	s.srv = server.NewMCPServer("cortex-cc", "1.0.0",
		server.WithToolCapabilities(true),
	)
	s.registerTools()
	return s
}

func (s *MCPServer) registerTools() {
	for _, def := range s.registry.Definitions() {
		def := def // capture
		tool := mcpsdk.NewTool(def.Function.Name,
			mcpsdk.WithDescription(def.Function.Description),
		)
		s.srv.AddTool(tool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args := map[string]any{}
			if req.Params.Arguments != nil {
				if b, ok := req.Params.Arguments.([]byte); ok {
					if err := json.Unmarshal(b, &args); err != nil {
						return mcpsdk.NewToolResultError(fmt.Sprintf("bad args: %v", err)), nil
					}
				} else if m, ok := req.Params.Arguments.(map[string]any); ok {
					args = m
				}
			}
			result, err := s.registry.Execute(def.Function.Name, args)
			if err != nil {
				return mcpsdk.NewToolResultError(err.Error()), nil
			}
			return mcpsdk.NewToolResultText(result), nil
		})
	}
	log.Printf("mcp: registered %d tools", len(s.registry.Definitions()))
}

// ServeStdio starts the MCP server over stdio — for use with Claude Desktop or any MCP host.
func (s *MCPServer) ServeStdio() error {
	return server.ServeStdio(s.srv)
}
