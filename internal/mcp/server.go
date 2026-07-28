package mcp

import (
	"context"
	"encoding/json"
	"log"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MCPServer wraps the MCP SDK and exposes the tool registry over the
// Model Context Protocol stdio transport, allowing any MCP-compatible
// client (Cursor, VS Code Copilot, etc.) to call contact center tools.
type MCPServer struct {
	exec Executor
	srv  *server.MCPServer
}

func NewMCPServer(exec Executor) *MCPServer {
	s := &MCPServer{exec: exec}
	s.srv = server.NewMCPServer("cortex-cc", "1.0.0",
		server.WithToolCapabilities(true),
	)
	s.registerTools()
	return s
}

func (s *MCPServer) registerTools() {
	for _, def := range s.exec.Definitions() {
		def := def
		tool := mcpsdk.NewTool(def.Function.Name,
			mcpsdk.WithDescription(def.Function.Description),
			mcpsdk.WithRawInputSchema(def.Function.Parameters),
		)
		s.srv.AddTool(tool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args := req.GetArguments()
			if args == nil {
				// fallback: try raw JSON arguments
				args = map[string]any{}
				if req.Params.RawArguments != nil {
					json.Unmarshal(req.Params.RawArguments, &args) //nolint:errcheck
				}
			}
			result, err := s.exec.Execute(def.Function.Name, args)
			if err != nil {
				return mcpsdk.NewToolResultError(err.Error()), nil
			}
			return mcpsdk.NewToolResultText(result), nil
		})
	}
	log.Printf("mcp: registered %d tools", len(s.exec.Definitions()))
}

// ServeStdio starts the MCP server over stdio — used by cmd/mcp-server.
func (s *MCPServer) ServeStdio() error {
	return server.ServeStdio(s.srv)
}
