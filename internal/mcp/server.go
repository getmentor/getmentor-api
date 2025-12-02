package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"go.uber.org/zap"
)

// Server represents an MCP server instance
type Server struct {
	mentorService services.MentorServiceInterface
	config        *config.Config
}

// NewServer creates a new MCP server
func NewServer(mentorService services.MentorServiceInterface, cfg *config.Config) *Server {
	return &Server{
		mentorService: mentorService,
		config:        cfg,
	}
}

// HandleRequest processes an MCP JSON-RPC request
func (s *Server) HandleRequest(ctx context.Context, req Request) Response {
	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		return s.errorResponse(req.ID, InvalidRequest, "Invalid JSON-RPC version")
	}

	// Route to appropriate handler based on method
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolCall(ctx, req)
	default:
		metrics.MCPErrors.WithLabelValues("method_not_found").Inc()
		return s.errorResponse(req.ID, MethodNotFound, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

// handleInitialize handles the initialize request
func (s *Server) handleInitialize(req Request) Response {
	logger.Info("MCP initialize request received")

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "getmentor-mcp-server",
			Version: "1.0.0",
		},
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleToolsList handles the tools/list request
func (s *Server) handleToolsList(req Request) Response {
	logger.Info("MCP tools/list request received")

	tools := []Tool{
		{
			Name:        "search_mentors",
			Description: "Search for mentors based on keywords and filters. Returns a list of mentors with basic information including name, job title, workplace, experience, competencies, price, and tags.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"keywords": {
						Type:        "array",
						Description: "Keywords to search in competencies, description, and about fields (case-insensitive)",
						Items:       &Items{Type: "string"},
					},
					"name": {
						Type:        "string",
						Description: "Filter by exact mentor name",
					},
					"workplace": {
						Type:        "string",
						Description: "Filter by exact workplace",
					},
					"experience": {
						Type:        "string",
						Description: "Filter by experience level",
					},
					"price": {
						Type:        "string",
						Description: "Filter by price",
					},
					"tags": {
						Type:        "array",
						Description: "Filter by tags (must match at least one tag)",
						Items:       &Items{Type: "string"},
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results to return (default: 50)",
						Default:     50,
					},
				},
			},
		},
		{
			Name:        "get_mentor_details",
			Description: "Get detailed information about a specific mentor including description and about sections. You must provide either id or slug.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"id": {
						Type:        "integer",
						Description: "Mentor ID",
					},
					"slug": {
						Type:        "string",
						Description: "Mentor slug (URL-friendly identifier)",
					},
				},
			},
		},
	}

	result := ToolsListResult{
		Tools: tools,
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleToolCall handles the tools/call request
func (s *Server) handleToolCall(ctx context.Context, req Request) Response {
	start := time.Now()

	// Parse params
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		metrics.MCPErrors.WithLabelValues("invalid_params").Inc()
		return s.errorResponse(req.ID, InvalidParams, "Invalid params format")
	}

	var params ToolCallParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		metrics.MCPErrors.WithLabelValues("invalid_params").Inc()
		return s.errorResponse(req.ID, InvalidParams, "Invalid params structure")
	}

	logger.Info("MCP tool call",
		zap.String("tool", params.Name),
		zap.Any("arguments", params.Arguments),
	)

	var result ToolCallResult
	var toolErr error

	// Route to appropriate tool
	switch params.Name {
	case "search_mentors":
		result, toolErr = s.searchMentors(ctx, params.Arguments)
		metrics.MCPToolDuration.WithLabelValues("search_mentors").Observe(metrics.MeasureDuration(start))
	case "get_mentor_details":
		result, toolErr = s.getMentorDetails(ctx, params.Arguments)
		metrics.MCPToolDuration.WithLabelValues("get_mentor_details").Observe(metrics.MeasureDuration(start))
	default:
		metrics.MCPErrors.WithLabelValues("tool_not_found").Inc()
		return s.errorResponse(req.ID, MethodNotFound, fmt.Sprintf("Tool not found: %s", params.Name))
	}

	if toolErr != nil {
		logger.Error("MCP tool execution failed",
			zap.String("tool", params.Name),
			zap.Error(toolErr),
		)

		// Determine error code based on error type
		errorCode := InternalError
		errorType := "internal_error"
		if strings.Contains(toolErr.Error(), "not found") {
			errorCode = NotFoundError
			errorType = "not_found"
		}

		metrics.MCPToolInvocations.WithLabelValues(params.Name, "error").Inc()
		metrics.MCPErrors.WithLabelValues(errorType).Inc()
		return s.errorResponse(req.ID, errorCode, toolErr.Error())
	}

	metrics.MCPToolInvocations.WithLabelValues(params.Name, "success").Inc()

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// errorResponse creates an error response
func (s *Server) errorResponse(id interface{}, code int, message string) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
}

// searchMentors implements the search_mentors tool
func (s *Server) searchMentors(ctx context.Context, args map[string]interface{}) (ToolCallResult, error) {
	// Parse arguments
	var searchArgs SearchMentorsArgs
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to parse arguments: %w", err)
	}
	if err := json.Unmarshal(argsJSON, &searchArgs); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid arguments structure: %w", err)
	}

	// Set default limit
	if searchArgs.Limit == 0 {
		searchArgs.Limit = 50
	}

	// Get all visible mentors
	mentors, err := s.mentorService.GetAllMentors(ctx, models.FilterOptions{
		OnlyVisible: true,
	})
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to fetch mentors: %w", err)
	}

	// Filter mentors based on search criteria
	filtered := s.filterMentors(mentors, searchArgs)

	// Limit results
	if len(filtered) > searchArgs.Limit {
		filtered = filtered[:searchArgs.Limit]
	}

	// Convert to search results
	results := make([]MentorSearchResult, 0, len(filtered))
	for _, mentor := range filtered {
		results = append(results, MentorSearchResult{
			ID:           mentor.ID,
			Name:         mentor.Name,
			JobTitle:     mentor.Job,
			Workplace:    mentor.Workplace,
			Experience:   mentor.Experience,
			Price:        mentor.Price,
			Tags:         mentor.Tags,
			Competencies: mentor.Competencies,
			Slug:         mentor.Slug,
		})
	}

	// Format results as JSON text
	resultsJSON, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to format results: %w", err)
	}

	logger.Info("MCP search_mentors completed",
		zap.Int("results_count", len(results)),
		zap.Any("search_args", searchArgs),
	)

	return ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: string(resultsJSON),
			},
		},
	}, nil
}

// filterMentors filters mentors based on search criteria
func (s *Server) filterMentors(mentors []*models.Mentor, args SearchMentorsArgs) []*models.Mentor {
	filtered := make([]*models.Mentor, 0)

	for _, mentor := range mentors {
		// Apply deterministic filters (exact match)
		if args.Name != "" && mentor.Name != args.Name {
			continue
		}
		if args.Workplace != "" && mentor.Workplace != args.Workplace {
			continue
		}
		if args.Experience != "" && mentor.Experience != args.Experience {
			continue
		}
		if args.Price != "" && mentor.Price != args.Price {
			continue
		}

		// Apply tags filter (must match at least one tag)
		if len(args.Tags) > 0 {
			hasMatchingTag := false
			for _, filterTag := range args.Tags {
				for _, mentorTag := range mentor.Tags {
					if filterTag == mentorTag {
						hasMatchingTag = true
						break
					}
				}
				if hasMatchingTag {
					break
				}
			}
			if !hasMatchingTag {
				continue
			}
		}

		// Apply keyword search (case-insensitive)
		if len(args.Keywords) > 0 {
			searchText := strings.ToLower(mentor.Competencies + " " + mentor.Description + " " + mentor.About)
			hasKeyword := false
			for _, keyword := range args.Keywords {
				if strings.Contains(searchText, strings.ToLower(keyword)) {
					hasKeyword = true
					break
				}
			}
			if !hasKeyword {
				continue
			}
		}

		filtered = append(filtered, mentor)
	}

	return filtered
}

// getMentorDetails implements the get_mentor_details tool
func (s *Server) getMentorDetails(ctx context.Context, args map[string]interface{}) (ToolCallResult, error) {
	// Parse arguments
	var detailsArgs GetMentorDetailsArgs
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to parse arguments: %w", err)
	}
	if err := json.Unmarshal(argsJSON, &detailsArgs); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid arguments structure: %w", err)
	}

	// Validate that either ID or Slug is provided
	if detailsArgs.ID == 0 && detailsArgs.Slug == "" {
		return ToolCallResult{}, fmt.Errorf("either id or slug must be provided")
	}

	// Fetch mentor
	var mentor *models.Mentor
	if detailsArgs.ID != 0 {
		mentor, err = s.mentorService.GetMentorByID(ctx, detailsArgs.ID, models.FilterOptions{OnlyVisible: true})
	} else {
		mentor, err = s.mentorService.GetMentorBySlug(ctx, detailsArgs.Slug, models.FilterOptions{OnlyVisible: true})
	}

	if err != nil {
		return ToolCallResult{}, fmt.Errorf("mentor not found")
	}

	// Convert to detailed result
	result := MentorDetailsResult{
		ID:           mentor.ID,
		Name:         mentor.Name,
		JobTitle:     mentor.Job,
		Workplace:    mentor.Workplace,
		Experience:   mentor.Experience,
		Price:        mentor.Price,
		Tags:         mentor.Tags,
		Competencies: mentor.Competencies,
		Description:  mentor.Description,
		About:        mentor.About,
		PhotoURL:     mentor.PhotoURL,
		Slug:         mentor.Slug,
		Link:         s.config.Server.BaseURL + "/mentor/" + mentor.Slug,
	}

	// Format result as JSON text
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to format result: %w", err)
	}

	logger.Info("MCP get_mentor_details completed",
		zap.Int("mentor_id", mentor.ID),
		zap.String("mentor_slug", mentor.Slug),
	)

	return ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: string(resultJSON),
			},
		},
	}, nil
}
