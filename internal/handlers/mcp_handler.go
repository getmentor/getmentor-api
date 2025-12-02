package handlers

import (
	"net/http"

	"github.com/getmentor/getmentor-api/internal/mcp"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MCPHandler handles MCP (Model Context Protocol) requests
type MCPHandler struct {
	server *mcp.Server
}

// NewMCPHandler creates a new MCP handler
func NewMCPHandler(server *mcp.Server) *MCPHandler {
	return &MCPHandler{
		server: server,
	}
}

// HandleMCP handles incoming MCP JSON-RPC requests
func (h *MCPHandler) HandleMCP(c *gin.Context) {
	var req mcp.Request

	// Parse JSON-RPC request
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("MCP invalid request format",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()),
		)
		c.JSON(http.StatusBadRequest, mcp.Response{
			JSONRPC: "2.0",
			Error: &mcp.RPCError{
				Code:    mcp.ParseError,
				Message: "Failed to parse JSON-RPC request",
			},
		})
		return
	}

	logger.Info("MCP request received",
		zap.String("method", req.Method),
		zap.Any("id", req.ID),
		zap.String("client_ip", c.ClientIP()),
	)

	// Process request
	response := h.server.HandleRequest(c.Request.Context(), req)

	// Determine HTTP status code based on response
	statusCode := http.StatusOK
	if response.Error != nil {
		switch response.Error.Code {
		case mcp.AuthenticationError:
			statusCode = http.StatusUnauthorized
		case mcp.RateLimitError:
			statusCode = http.StatusTooManyRequests
		case mcp.NotFoundError:
			statusCode = http.StatusNotFound
		case mcp.InvalidRequest, mcp.InvalidParams:
			statusCode = http.StatusBadRequest
		default:
			statusCode = http.StatusInternalServerError
		}

		logger.Warn("MCP request error",
			zap.String("method", req.Method),
			zap.Int("error_code", response.Error.Code),
			zap.String("error_message", response.Error.Message),
		)
	}

	c.JSON(statusCode, response)
}
