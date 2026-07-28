package main

import (
	"log"
	"os"

	"cortex-cc/internal/mcp"
)

// cortex-mcp-server is a standalone MCP stdio server.
// It connects to a running cortex-cc instance via HTTP and exposes
// all 7 contact center tools over the Model Context Protocol,
// letting any MCP client (Cursor, VS Code Copilot, etc.) query
// and control the contact center in plain English.
//
// Usage:
//
//	CORTEX_URL=http://localhost:8080 ./bin/cortex-mcp
//
// Cursor config (~/.cursor/mcp.json):
//
//	{
//	  "mcpServers": {
//	    "cortex-cc": {
//	      "command": "/path/to/bin/cortex-mcp",
//	      "env": { "CORTEX_URL": "http://localhost:8080" }
//	    }
//	  }
//	}
func main() {
	// MCP stdio protocol uses stdout — send all logs to stderr.
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime)

	baseURL := os.Getenv("CORTEX_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	log.Printf("cortex-mcp: connecting to %s", baseURL)

	exec := mcp.NewHTTPExecutor(baseURL)
	srv := mcp.NewMCPServer(exec)

	if err := srv.ServeStdio(); err != nil {
		log.Fatalf("mcp server error: %v", err)
	}
}
