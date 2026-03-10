package mcp

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/mcp/protocol"
	"github.com/liza-mas/liza/internal/paths"
)

// Server represents the MCP server
type Server struct {
	projectRoot string
	logPath     string
	logger      *slog.Logger
	bb          *db.Blackboard
	tools       map[string]protocol.Tool
	resources   map[string]protocol.Resource
	handlers    map[string]ToolHandler
}

// NewServer creates a new MCP server
func NewServer(projectRoot, logPath string) *Server {
	s := &Server{
		projectRoot: projectRoot,
		logPath:     logPath,
		logger:      slog.New(slog.NewTextHandler(os.Stderr, nil)),
		bb:          db.For(paths.New(projectRoot).StatePath()),
		tools:       make(map[string]protocol.Tool),
		resources:   make(map[string]protocol.Resource),
		handlers:    make(map[string]ToolHandler),
	}

	s.registerReadOnlyTools()
	s.registerReadOnlyResources()
	s.registerMutationTools()
	s.registerComplexOperations()

	return s
}

// GetCapabilities returns the server capabilities
func (s *Server) GetCapabilities() map[string]any {
	return map[string]any{
		"tools": map[string]any{
			"listChanged": false,
		},
		"resources": map[string]any{
			"subscribe":   false,
			"listChanged": false,
		},
	}
}

// ListTools returns all registered tools
func (s *Server) ListTools() []protocol.Tool {
	tools := make([]protocol.Tool, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetTool returns a specific tool by name
func (s *Server) GetTool(name string) (protocol.Tool, bool) {
	tool, ok := s.tools[name]
	return tool, ok
}

// GetHandler returns a specific handler by tool name
func (s *Server) GetHandler(name string) (ToolHandler, bool) {
	handler, ok := s.handlers[name]
	return handler, ok
}

// ToolNames returns all registered tool names
func (s *Server) ToolNames() []string {
	names := make([]string, 0, len(s.tools))
	for name := range s.tools {
		names = append(names, name)
	}
	return names
}

// ListResources returns all registered resources
func (s *Server) ListResources() []protocol.Resource {
	resources := make([]protocol.Resource, 0, len(s.resources))
	for _, resource := range s.resources {
		resources = append(resources, resource)
	}
	return resources
}

// Run starts the MCP server with stdio transport
func (s *Server) Run() error {
	return s.runWithTransport(protocol.NewStdioTransport())
}

func (s *Server) runWithTransport(transport runTransport) error {
	for {
		req, err := transport.ReadRequest()
		if err != nil {
			// EOF means client disconnected, exit gracefully
			if errors.Is(err, io.EOF) || err.Error() == "EOF" {
				return nil
			}
			// Use appropriate error code: RequestTooLarge for size violations, ParseError for others
			errorCode := protocol.ParseError
			if errors.Is(err, protocol.ErrRequestTooLarge) {
				errorCode = protocol.RequestTooLarge
			}
			if writeErr := transport.WriteError(nil, errorCode, err.Error(), nil); writeErr != nil {
				return fmt.Errorf("failed to write error response: %w", writeErr)
			}
			continue
		}

		// JSON-RPC 2.0: requests without an "id" field are notifications.
		// The server MUST NOT reply to notifications.
		if req.ID == nil {
			s.handleNotification(req)
			continue
		}

		resp := s.HandleRequest(req)
		if err := transport.WriteResponse(resp); err != nil {
			return fmt.Errorf("failed to write response: %w", err)
		}
	}
}
